package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// jsonEnvelope 是所有 --json 输出的统一外壳：ok 表示命令是否成功，error 只在
// 失败时出现，data 是命令自己的结构化结果。
type jsonEnvelope struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Data  any    `json:"data,omitempty"`
}

// jsonMode 读取全局 --json；未注册 flag（直接调用 RunE 的测试）时按关闭处理。
func jsonMode() bool {
	flag := pflag.Lookup("json")
	return flag != nil && flag.Value.String() == "true"
}

// printJSON 输出一条成功结果。
func printJSON(cmd *cobra.Command, data any) error {
	payload, err := json.MarshalIndent(jsonEnvelope{OK: true, Data: data}, "", "  ")
	if err != nil {
		return err
	}
	_, err = cmd.OutOrStdout().Write(append(payload, '\n'))
	return err
}

// planChangeJSON / planTableJSON / planJSON 是 crud:apply --plan 与
// migrate --plan 的机器可读契约；agent 据此决定是否需要人工批准或写业务迁移。
type planChangeJSON struct {
	Field     string   `json:"field"`
	Type      string   `json:"type,omitempty"`
	Class     string   `json:"class"`
	Category  string   `json:"category,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	DDL       string   `json:"ddl,omitempty"`
	Approved  bool     `json:"approved"`
	Unmanaged []string `json:"unmanaged,omitempty"`
}

type planTableJSON struct {
	Table       string           `json:"table"`
	Action      string           `json:"action"`
	Destructive bool             `json:"destructive,omitempty"`
	Changes     []planChangeJSON `json:"changes"`
	Notes       []string         `json:"notes,omitempty"`
	Blocked     int              `json:"blocked"`
}

type planJSON struct {
	Tables            []planTableJSON `json:"tables"`
	BlockedCategories []string        `json:"blockedCategories,omitempty"`
	ApproveCommand    string          `json:"approveCommand,omitempty"`
}

// planToJSON 把 apply/plan 结果转成契约结构。specRef 是传给 crud:apply 的
// 路径或目录，用来生成可直接执行的 approveCommand。
func planToJSON(results []helper.ApplyTableResult, specRef string) planJSON {
	payload := planJSON{Tables: make([]planTableJSON, 0, len(results))}
	categorySet := map[string]struct{}{}
	for _, result := range results {
		table := planTableJSON{
			Table:       result.Table,
			Action:      string(result.Action),
			Destructive: result.Destructive,
			Changes:     make([]planChangeJSON, 0, len(result.Diffs)),
			Notes:       make([]string, 0, len(result.Unmanaged)),
			Blocked:     len(result.Blocked),
		}
		for _, change := range result.Diffs {
			item := planChangeJSON{
				Field: change.Field, Type: change.Type, Class: string(change.Class),
				Category: string(change.Category), Reason: change.Reason, DDL: change.DDL,
				Approved: change.Approved, Unmanaged: change.Unmanaged,
			}
			table.Changes = append(table.Changes, item)
			if change.Class == helper.DiffRequiresApproval && change.Category != "" && !change.Approved {
				categorySet[string(change.Category)] = struct{}{}
			}
		}
		for _, note := range result.Unmanaged {
			table.Notes = append(table.Notes, note.Field+": "+note.Reason)
		}
		payload.Tables = append(payload.Tables, table)
	}
	for category := range categorySet {
		payload.BlockedCategories = append(payload.BlockedCategories, category)
	}
	sort.Strings(payload.BlockedCategories)
	if len(payload.BlockedCategories) > 0 {
		ref := specRef
		if ref == "" {
			ref = "crud_specs/"
		}
		payload.ApproveCommand = fmt.Sprintf("crud:apply %s --approve=%s", ref, strings.Join(payload.BlockedCategories, ","))
	}
	return payload
}

type specWarningJSON struct {
	Spec    string `json:"spec"`
	Message string `json:"message"`
}

type validateJSON struct {
	Validated int               `json:"validated"`
	Warnings  []specWarningJSON `json:"warnings"`
}

// printValidateJSON 输出 crud:validate 的结果；校验失败时由 executeRoot 统一
// 输出错误信封。
func printValidateJSON(cmd *cobra.Command, paths []string) error {
	warnings, err := helper.ValidateSpecs(paths)
	if err != nil {
		return err
	}
	payload := validateJSON{Validated: len(paths), Warnings: make([]specWarningJSON, 0, len(warnings))}
	for _, warning := range warnings {
		payload.Warnings = append(payload.Warnings, specWarningJSON{Spec: warning.SpecPath, Message: warning.Message})
	}
	return printJSON(cmd, payload)
}

type migratePlanJSON struct {
	State      string   `json:"state"`
	Pending    int      `json:"pending"`
	Incomplete int      `json:"incomplete"`
	Plan       planJSON `json:"plan"`
}

type generatedJSON struct {
	LogID    int32    `json:"logId"`
	Files    []string `json:"files"`
	Warnings []string `json:"warnings,omitempty"`
}

type dryRunJSON struct {
	Plan      planJSON `json:"plan"`
	Create    []string `json:"filesToCreate"`
	Overwrite []string `json:"filesToOverwrite"`
}
