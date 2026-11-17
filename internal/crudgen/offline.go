package crudgen

import (
	"buildadmin-go/internal/conf"
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/pkg/dbschema"
	"buildadmin-go/internal/pkg/util"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// DirectorySink 把生成产物写到目标目录（保持仓库相对路径），用于离线渲染。
type DirectorySink struct {
	root  string
	mu    sync.Mutex
	files []string
}

// NewDirectorySink 创建目录 sink；root 为空时使用 runtime/offline。
func NewDirectorySink(root string) *DirectorySink {
	return &DirectorySink{root: root}
}

// WriteFile 把内容写到 <root>/<仓库相对路径>。
func (s *DirectorySink) WriteFile(path, content string) error {
	relative, err := filepath.Rel(util.RootPath(), path)
	if err != nil || strings.HasPrefix(relative, "..") || filepath.IsAbs(relative) {
		return fmt.Errorf("offline render target %s is outside the repository", path)
	}
	target := filepath.Join(s.root, relative)
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files = append(s.files, relative)
	return nil
}

// Paths 返回已写出的仓库相对路径（排序）。
func (s *DirectorySink) Paths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	paths := append([]string(nil), s.files...)
	sort.Strings(paths)
	return paths
}

// offlineTableStore 是无数据库时的表元数据桩：只提供表名，列信息为空。生成器
// 在离线模式下已拒绝依赖列元数据的关联字段，因此空列不会影响产物。
type offlineTableStore struct {
	prefix string
}

func (s offlineTableStore) Name(name string, full bool) string {
	if full {
		return s.prefix + name
	}
	return name
}

func (s offlineTableStore) GetColumns(string) ([]dbschema.Column, error) { return nil, nil }

func (s offlineTableStore) GetTableCollation(string) (string, error) { return "", nil }

func (s offlineTableStore) DelTable(string) error {
	return fmt.Errorf("offline render cannot modify tables")
}

// RenderOffline 不连数据库，把 spec 渲染成产物写入 outDir，返回仓库相对路径。
//
// 适用于"还没有库、先看会生成什么"。实体、仓库、DTO、handler、registrar 与前端
// 产物都由 spec 推导，与在线生成逐字节一致；需要数据库列元数据的 remoteSelect
// 关联会被拒绝（否则会静默产出降级的关联配置）。
func RenderOffline(specPath, outDir, prefix string) ([]string, string, error) {
	options, err := LoadSpec(specPath)
	if err != nil {
		return nil, "", err
	}
	if options.Table.RegisterOnly {
		return nil, "", fmt.Errorf("spec %q is registerOnly; there is nothing to render", specPath)
	}
	if err := rejectOfflineRelations(options.Fields); err != nil {
		return nil, "", err
	}
	// reassignable 会把 owner 列预置成 admin 的远程下拉，展开它需要 admin 表的
	// 真实列元数据（pk 类型、username 长度）；无库时无法保证与在线产物一致，
	// 明确拒绝而不是静默产出降级的关联配置。
	if options.Table.DataScope != nil && options.Table.DataScope.Reassignable {
		return nil, "", fmt.Errorf("offline rendering cannot expand the reassignable owner's admin relation: it needs admin column metadata from the database; use crud:generate --dry-run against a database or drop dataScope.reassignable")
	}
	if strings.TrimSpace(prefix) == "" {
		prefix = "ba_"
	}
	if strings.TrimSpace(outDir) == "" {
		outDir = filepath.Join(util.RootPath(), "runtime", "offline", options.Table.Name)
	}
	absoluteOut, err := filepath.Abs(outDir)
	if err != nil {
		return nil, "", err
	}
	sink := NewDirectorySink(absoluteOut)
	state := &generateState{
		cfg:    &conf.Configuration{Database: conf.Database{Prefix: prefix}},
		tables: offlineTableStore{prefix: prefix},
		opts:   *options,
	}
	state.opts.AdminID = 1
	state.opts.PreviewSink = sink
	if early, err := state.prepareSpecInputs(); err != nil {
		return nil, "", err
	} else if early != nil {
		return nil, "", earlyError(early)
	}
	if err := state.prepareInheritance(); err != nil {
		return nil, "", err
	}
	if _, err := state.renderPreview(sink); err != nil {
		return nil, "", err
	}
	return sink.Paths(), absoluteOut, nil
}

// earlyError 把准备阶段的提前结果转成错误（离线模式不允许 registerOnly 等短路）。
func earlyError(result *GenerateResult) error {
	if result == nil {
		return nil
	}
	return fmt.Errorf("offline render was short-circuited by the generator")
}

// rejectOfflineRelations 拒绝依赖数据库列元数据的关联字段。
func rejectOfflineRelations(fields []crudmodel.Field) error {
	blocked := make([]string, 0)
	for _, field := range fields {
		if field.Form.RemoteTable == "" {
			continue
		}
		if field.DesignType == "remoteSelect" || field.DesignType == "remoteSelects" {
			blocked = append(blocked, field.Name+" -> "+field.Form.RemoteTable)
		}
	}
	if len(blocked) > 0 {
		return fmt.Errorf("offline render cannot resolve relation columns for %s; run the generation against a database (remoteSelect needs the remote table's columns)", strings.Join(blocked, ", "))
	}
	return nil
}
