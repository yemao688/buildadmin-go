package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"buildadmin-go/internal/model"
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/util"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

// newCrudExportSpecCommand 把历史设计器生成记录（crud_log）导出为 spec 文件。
// 设计器在本次改动之后才写 spec，存量模块用这条命令补齐，避免它们对
// crud:apply / migrate --plan 不可见。
func newCrudExportSpecCommand(cmdBootstrap CmdBootstrap) *cobra.Command {
	var (
		targetDir string
		force     bool
	)
	exportCmd := &cobra.Command{
		Use:           "crud:export-spec <table>",
		Short:         "把 crud_log 中的成功生成记录导出为 crud_specs/<table>.yaml",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: withCmd(cmdBootstrap, func(cmd *cobra.Command, command *Command, args []string) error {
			return command.crudH.ExportSpec(cmd, args[0], targetDir, force)
		}),
	}
	exportCmd.Flags().StringVar(&targetDir, "dir", "", "spec 输出目录，默认 crud_specs/")
	exportCmd.Flags().BoolVar(&force, "force", false, "覆盖已存在的 spec 文件")
	return exportCmd
}

// ExportSpec 读取该表最近一次成功的 crud_log 并写回 spec；产物先自检再落盘，
// 校验失败时删除文件，不留半成品。
func (h *CrudHandler) ExportSpec(cmd *cobra.Command, table, targetDir string, force bool) error {
	if err := data_scope.ValidateIdentifier(table); err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	var record model.Log
	err := h.db.Table(h.config.Database.Prefix+"crud_log").
		Where("table_name = ? AND status = ?", table, "success").
		Order("id DESC").
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("no successful crud_log for table %q; generate it from the designer once or write the spec by hand", table)
	}
	if err != nil {
		return fmt.Errorf("read crud_log: %w", err)
	}
	if len(record.Fields) == 0 {
		return fmt.Errorf("crud_log for table %q has no field design", table)
	}
	dir := targetDir
	if dir == "" {
		dir = helper.DefaultSpecDir()
	}
	if dir == "" {
		dir = filepath.Join(util.RootPath(), "crud_specs")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create spec directory: %w", err)
	}
	path := filepath.Join(dir, table+".yaml")
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("spec %s already exists; pass --force to overwrite", path)
	}
	content, err := helper.MarshalSpec(crudmodel.Table(record.Table), []crudmodel.Field(record.Fields))
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, content, 0o644); err != nil { //nolint:gosec // G306: spec 是仓库内可读文件
		return fmt.Errorf("write spec: %w", err)
	}
	if _, err := helper.ValidateSpec(path); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("exported spec failed validation: %w", err)
	}
	cmd.Printf("exported %s\n", path)
	cmd.Printf("review it, then use crud:generate %s (the designer is no longer required)\n", path)
	return nil
}
