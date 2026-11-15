package crudgen

import (
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/pkg/util"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FileManifest struct {
	Generated []string
	Shared    []string
}

// BuildFileManifest returns every known CRUD output plus the shared files that
// Wire/provider updates can modify. DDL is intentionally not part of this
// transaction: MySQL DDL cannot be rolled back reliably.
func BuildFileManifest(table crudmodel.Table) (FileManifest, error) {
	if err := normalizeTableConfiguration(&table); err != nil {
		return FileManifest{}, err
	}
	entityFile, err := ParseEntityNameData(table.Name, table.ModelFile)
	if err != nil {
		return FileManifest{}, err
	}
	repositoryFile, err := ParseRepositoryNameData(table.Name, table.ModelFile)
	if err != nil {
		return FileManifest{}, err
	}
	dtoFile, err := ParseDTONameData(table.Name, table.ModelFile)
	if err != nil {
		return FileManifest{}, err
	}
	handlerFile, err := ParseHandlerNameData(table.Name, table.ControllerFile)
	if err != nil {
		return FileManifest{}, err
	}
	registrarFile, err := ParseRegistrarNameData(table.Name, table.ControllerFile)
	if err != nil {
		return FileManifest{}, err
	}
	views := ParseWebDirNameData(table.Name, "views", table.WebViewsDir)
	lang := ParseWebDirNameData(table.Name, "lang", table.WebViewsDir)
	manifest := FileManifest{
		Generated: []string{
			filepath.Join(util.RootPath(), lang.LangFile("en")),
			filepath.Join(util.RootPath(), lang.LangFile("zh-cn")),
			filepath.Join(util.RootPath(), views.Views, "index.vue"),
			filepath.Join(util.RootPath(), views.Views, "popupForm.vue"),
			entityFile.ParseFile,
			repositoryFile.ParseFile,
			dtoFile.ParseFile,
			handlerFile.ParseFile,
			registrarFile.ParseFile,
		},
		Shared: []string{
			// 拍平布局：构造器并入扁平包合并 ProviderSet，wire.go 不再被修改；
			// wire_gen.go 由生成流程重新生成。
			filepath.Join(util.RootPath(), repositoryFile.RootFileName, "provider.go"),
			filepath.Join(util.RootPath(), handlerFile.RootFileName, "provider.go"),
			filepath.Join(util.RootPath(), "internal", "admin", "router", "provider.go"),
			filepath.Join(util.RootPath(), "cmd", "server", "wire_gen.go"),
		},
	}
	for _, path := range manifest.Generated {
		if err := ValidateGeneratedAbsolutePath(path, "web/src/lang", "web/src/views", "internal/model", "internal/admin/repository", "internal/admin/dto", "internal/admin/handler", "internal/admin/router"); err != nil {
			return FileManifest{}, err
		}
	}
	for _, path := range manifest.Shared {
		if err := ValidateGeneratedAbsolutePath(path, "internal/admin/repository", "internal/admin/handler", "internal/admin/router", "cmd/server"); err != nil {
			return FileManifest{}, err
		}
	}
	return normalizeFileManifest(manifest)
}

func BuildFileManifestForFields(table crudmodel.Table, fields []crudmodel.Field) (FileManifest, error) {
	manifest, err := BuildFileManifest(table)
	if err != nil {
		return FileManifest{}, err
	}
	for _, field := range fields {
		if field.Form.RemoteTable == "" || field.Form.RelationFields == "" {
			continue
		}
		join, err := ParseEntityNameData(field.Form.RemoteTable, field.Form.RemoteModel)
		if err != nil {
			return FileManifest{}, err
		}
		if !fileExists(join.ParseFile) {
			manifest.Generated = append(manifest.Generated, join.ParseFile)
		}
		joinRepo, err := ParseRepositoryNameData(field.Form.RemoteTable, field.Form.RemoteModel)
		if err != nil {
			return FileManifest{}, err
		}
		provider := filepath.Join(util.RootPath(), joinRepo.RootFileName, "provider.go")
		manifest.Shared = append(manifest.Shared, provider)
	}
	manifest, err = normalizeFileManifest(manifest)
	if err != nil {
		return FileManifest{}, err
	}
	for _, path := range append(append([]string{}, manifest.Generated...), manifest.Shared...) {
		if err := ValidateGeneratedAbsolutePath(path, "web/src/lang", "web/src/views", "internal/model", "internal/admin/repository", "internal/admin/dto", "internal/admin/handler", "internal/admin/router", "internal", "cmd/server"); err != nil {
			return FileManifest{}, err
		}
	}
	return manifest, nil
}

type FileSnapshot struct {
	dir     string
	entries []snapshotEntry
}

type snapshotEntry struct {
	path    string
	backup  string
	existed bool
	mode    os.FileMode
}

func NewFileSnapshot(paths []string) (*FileSnapshot, error) {
	dir, err := os.MkdirTemp(util.RootPath(), ".buildadmin-crud-backup-")
	if err != nil {
		return nil, err
	}
	s := &FileSnapshot{dir: dir}
	for i, path := range uniquePaths(paths) {
		info, statErr := os.Stat(path)
		entry := snapshotEntry{path: path}
		if statErr == nil {
			if !info.Mode().IsRegular() {
				_ = s.Cleanup()
				return nil, fmt.Errorf("snapshot target is not a regular file: %s", path)
			}
			entry.existed = true
			entry.mode = info.Mode().Perm()
			entry.backup = filepath.Join(dir, fmt.Sprintf("%06d.bak", i))
			data, readErr := os.ReadFile(path) //nolint:gosec // G304: path 为本次生成事务登记的待改写产物绝对路径，均来自 util.RootPath() 下的生成目录
			if readErr != nil {
				_ = s.Cleanup()
				return nil, readErr
			}
			if writeErr := os.WriteFile(entry.backup, data, entry.mode); writeErr != nil { //nolint:gosec // G703: 备份文件位于 NewFileSnapshot 用 os.MkdirTemp 创建的临时目录，文件名由序号格式化为 %06d.bak
				_ = s.Cleanup()
				return nil, writeErr
			}
		} else if !os.IsNotExist(statErr) {
			_ = s.Cleanup()
			return nil, statErr
		}
		s.entries = append(s.entries, entry)
	}
	return s, nil
}

func (s *FileSnapshot) Restore() error {
	prepared := make([]struct {
		entry snapshotEntry
		data  []byte
	}, 0, len(s.entries))
	for _, entry := range s.entries {
		if !entry.existed {
			prepared = append(prepared, struct {
				entry snapshotEntry
				data  []byte
			}{entry: entry})
			continue
		}
		data, err := os.ReadFile(entry.backup)
		if err != nil {
			return fmt.Errorf("read snapshot backup %q: %w", entry.backup, err)
		}
		prepared = append(prepared, struct {
			entry snapshotEntry
			data  []byte
		}{entry: entry, data: data})
	}
	var firstErr error
	for _, item := range prepared {
		entry := item.entry
		if !entry.existed {
			if err := os.Remove(entry.path); err != nil && !os.IsNotExist(err) && firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := atomicRestore(entry.path, item.data, entry.mode); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *FileSnapshot) Cleanup() error { return os.RemoveAll(s.dir) }

type Quarantine struct {
	dir     string
	entries []quarantineEntry
}

type quarantineEntry struct{ original, quarantined string }

func NewQuarantine(paths []string) (*Quarantine, error) {
	dir, err := os.MkdirTemp(util.RootPath(), ".buildadmin-crud-quarantine-")
	if err != nil {
		return nil, err
	}
	q := &Quarantine{dir: dir}
	for i, path := range uniquePaths(paths) {
		if _, err := os.Stat(path); err != nil {
			_ = q.Restore()
			_ = q.Commit()
			return nil, err
		}
		target := filepath.Join(dir, fmt.Sprintf("%06d.quarantine", i))
		if err := os.Rename(path, target); err != nil {
			_ = q.Restore()
			_ = q.Commit()
			return nil, err
		}
		q.entries = append(q.entries, quarantineEntry{original: path, quarantined: target})
	}
	return q, nil
}

func (q *Quarantine) Restore() error {
	var firstErr error
	for i := len(q.entries) - 1; i >= 0; i-- {
		entry := q.entries[i]
		if _, err := os.Stat(entry.quarantined); err == nil {
			if err := os.Rename(entry.quarantined, entry.original); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (q *Quarantine) Commit() error { return os.RemoveAll(q.dir) }

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; !ok {
			seen[path] = struct{}{}
			result = append(result, path)
		}
	}
	return result
}

// manifestRootDirs 列出 crud_log 清单可能记录的产物根（相对仓库根）。
// 旧记录保存的是生成时的绝对路径，读取时必须先按这些根重新挂到当前仓库，
// 才能在另一个 checkout / git worktree 里比较或删除同一份清单。
var manifestRootDirs = []string{
	"internal/model",
	"internal/admin/repository",
	"internal/admin/dto",
	"internal/admin/handler",
	"internal/admin/router",
	"web/src/lang",
	"web/src/views",
	"cmd/server",
}

// manifestPathForRoot 把清单路径解析到 root 下：相对路径直接拼接；绝对
// 路径已位于 root 下时原样返回，否则按 manifestRootDirs 截取产物根后缀并
// 重新挂到 root。无法识别根的旧绝对路径保持原样，由调用方校验拒绝。
func manifestPathForRoot(path, root string) string {
	clean := filepath.Clean(filepath.FromSlash(path))
	if !filepath.IsAbs(clean) {
		return filepath.Join(root, clean)
	}
	if manifestPathWithinRoot(clean, root) {
		return clean
	}
	if relative, ok := manifestRelativePath(clean); ok {
		return filepath.Join(root, filepath.FromSlash(relative))
	}
	return clean
}

// manifestPathKey 是清单比较的统一键：先按 root 解析，再规范化分隔符。
func manifestPathKey(path, root string) string {
	return filepath.Clean(canonicalManifestLangPath(manifestPathForRoot(path, root)))
}

func manifestPathWithinRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// manifestRelativePath 从绝对路径中截取已知产物根起点的相对路径。
// 取最靠右的匹配：checkout 目录名本身可能包含 internal/model 之类的片段，
// 真正的产物根一定最接近文件名。
func manifestRelativePath(path string) (string, bool) {
	slash := filepath.ToSlash(filepath.Clean(path))
	bestIndex := -1
	for _, root := range manifestRootDirs {
		index := strings.LastIndex(slash, "/"+root+"/")
		if index < 0 && strings.HasPrefix(slash, root+"/") {
			index = 0
		}
		if index > bestIndex {
			bestIndex = index
		}
	}
	switch {
	case bestIndex < 0:
		return "", false
	case bestIndex == 0:
		return slash, true
	default:
		return slash[bestIndex+1:], true
	}
}

// portableManifestPath 把当前仓库根下的绝对清单路径转为相对路径，供
// crud_log 持久化；跨 checkout 共享数据库时不再绑定生成时的根目录。
func portableManifestPath(path, root string) string {
	clean := filepath.Clean(filepath.FromSlash(path))
	if !filepath.IsAbs(clean) {
		return filepath.ToSlash(clean)
	}
	if !manifestPathWithinRoot(clean, root) {
		return filepath.ToSlash(clean)
	}
	relative, err := filepath.Rel(root, clean)
	if err != nil {
		return filepath.ToSlash(clean)
	}
	return filepath.ToSlash(relative)
}

func portableManifestPaths(paths []string) []string {
	root := util.RootPath()
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		result = append(result, portableManifestPath(path, root))
	}
	return result
}

// portableTableForLog 返回写入 crud_log 的 table 副本：manifest 路径统一
// 存相对仓库根形式，重新生成/删除时再按当前 util.RootPath() 还原。
func portableTableForLog(table crudmodel.Table) crudmodel.Table {
	table.GeneratedFiles = portableManifestPaths(table.GeneratedFiles)
	if table.Manifest != nil {
		table.Manifest = &crudmodel.CRUDFileManifest{
			Generated: portableManifestPaths(table.Manifest.Generated),
			Shared:    portableManifestPaths(table.Manifest.Shared),
		}
	}
	return table
}

// normalizeFileManifest is the single classification boundary for manifests,
// including manifests loaded from older CRUD logs.
func normalizeFileManifest(manifest FileManifest) (FileManifest, error) {
	generated := make([]string, 0, len(manifest.Generated))
	shared := append([]string{}, manifest.Shared...)
	for _, path := range manifest.Generated {
		if manifestPathIsShared(path) {
			shared = append(shared, path)
		} else {
			generated = append(generated, path)
		}
	}
	return FileManifest{Generated: uniquePaths(generated), Shared: uniquePaths(shared)}, nil
}

func manifestPathIsShared(path string) bool {
	clean := filepath.Clean(filepath.FromSlash(path))
	if !filepath.IsAbs(clean) {
		clean = filepath.Join(util.RootPath(), clean)
	}
	clean, _ = filepath.Abs(clean)
	root := filepath.Clean(util.RootPath())
	return filepath.Base(clean) == "provider.go" ||
		clean == filepath.Join(root, "cmd", "server", "wire.go") ||
		clean == filepath.Join(root, "cmd", "server", "wire_gen.go")
}

func atomicRestore(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".crud-restore-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err == nil {
		return nil
	}
	rollback, err := os.CreateTemp(filepath.Dir(path), ".crud-rollback-*")
	if err != nil {
		return err
	}
	rollbackName := rollback.Name()
	if err := rollback.Close(); err != nil {
		_ = os.Remove(rollbackName)
		return err
	}
	_ = os.Remove(rollbackName)
	if err := os.Rename(path, rollbackName); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		if rollbackErr := os.Rename(rollbackName, path); rollbackErr != nil {
			return fmt.Errorf("replace %s: %w; rollback failed: %w", path, err, rollbackErr)
		}
		return err
	}
	return os.Remove(rollbackName)
}
