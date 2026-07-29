package crud_helper

import (
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/utils"
	"os"
	"path/filepath"
)

type FileManifest struct {
	Generated []string
	Shared    []string
}

// BuildFileManifest returns every known CRUD output plus the shared files that
// Wire/provider updates can modify. DDL is intentionally not part of this
// transaction: MySQL DDL cannot be rolled back reliably.
func BuildFileManifest(table model.Table) (FileManifest, error) {
	if err := normalizeTableConfiguration(&table); err != nil {
		return FileManifest{}, err
	}
	module := "admin"
	modelRoot := "app/admin/model"
	if table.IsCommonModel != 0 {
		module = "common"
		modelRoot = "app/common/model"
	}
	modelFile, err := ParseNameData(module, table.Name, "model", table.ModelFile)
	if err != nil {
		return FileManifest{}, err
	}
	handlerFile, err := ParseNameData("admin", table.Name, "handler", table.ControllerFile)
	if err != nil {
		return FileManifest{}, err
	}
	views := ParseWebDirNameData(table.Name, "views", table.WebViewsDir)
	lang := ParseWebDirNameData(table.Name, "lang", table.WebViewsDir)
	manifest := FileManifest{
		Generated: []string{
			filepath.Join(utils.RootPath(), lang.LangFile("en")),
			filepath.Join(utils.RootPath(), lang.LangFile("zh-cn")),
			filepath.Join(utils.RootPath(), views.Views, "index.vue"),
			filepath.Join(utils.RootPath(), views.Views, "popupForm.vue"),
			modelFile.ParseFile,
			handlerFile.ParseFile,
			registrarFilePath(handlerFile),
		},
	Shared: []string{
		filepath.Join(utils.RootPath(), modelFile.RootFileName, "provider.go"),
		filepath.Join(utils.RootPath(), handlerFile.RootFileName, "provider.go"),
		filepath.Join(utils.RootPath(), "router", "registrar_set.go"),
		filepath.Join(utils.RootPath(), "cmd", "app", "wire.go"),
		filepath.Join(utils.RootPath(), "cmd", "app", "wire_gen.go"),
	},
	}
	for _, path := range manifest.Generated {
		if err := ValidateGeneratedAbsolutePath(path, "web/src/lang", "web/src/views", modelRoot, "app/admin/handler"); err != nil {
			return FileManifest{}, err
		}
	}
	for _, path := range manifest.Shared {
		if err := ValidateGeneratedAbsolutePath(path, "app", "router", "cmd/app"); err != nil {
			return FileManifest{}, err
		}
	}
	return normalizeFileManifest(manifest)
}

func BuildFileManifestForFields(table model.Table, fields []model.Field) (FileManifest, error) {
	manifest, err := BuildFileManifest(table)
	if err != nil {
		return FileManifest{}, err
	}
	for _, field := range fields {
		if field.Form.RemoteTable == "" || field.Form.RelationFields == "" {
			continue
		}
		join, err := ParseNameData("admin", field.Form.RemoteTable, "model", field.Form.RemoteModel)
		if err != nil {
			return FileManifest{}, err
		}
		if !fileExists(join.ParseFile) {
			manifest.Generated = append(manifest.Generated, join.ParseFile)
		}
		provider := filepath.Join(utils.RootPath(), join.RootFileName, "provider.go")
		manifest.Shared = append(manifest.Shared, provider)
	}
	manifest, err = normalizeFileManifest(manifest)
	if err != nil {
		return FileManifest{}, err
	}
	modelRoot := "app/admin/model"
	if table.IsCommonModel != 0 {
		modelRoot = "app/common/model"
	}
	for _, path := range append(append([]string{}, manifest.Generated...), manifest.Shared...) {
		if err := ValidateGeneratedAbsolutePath(path, "web/src/lang", "web/src/views", modelRoot, "app/admin/model", "app/admin/handler", "app/common/model", "app", "router", "cmd/app"); err != nil {
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
	dir, err := os.MkdirTemp(utils.RootPath(), ".buildadmin-crud-backup-")
	if err != nil {
		return nil, err
	}
	s := &FileSnapshot{dir: dir}
	for i, path := range uniquePaths(paths) {
		info, statErr := os.Stat(path)
		entry := snapshotEntry{path: path}
		if statErr == nil {
			if !info.Mode().IsRegular() {
				s.Cleanup()
				return nil, fmt.Errorf("snapshot target is not a regular file: %s", path)
			}
			entry.existed = true
			entry.mode = info.Mode().Perm()
			entry.backup = filepath.Join(dir, fmt.Sprintf("%06d.bak", i))
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				s.Cleanup()
				return nil, readErr
			}
			if writeErr := os.WriteFile(entry.backup, data, entry.mode); writeErr != nil {
				s.Cleanup()
				return nil, writeErr
			}
		} else if !os.IsNotExist(statErr) {
			s.Cleanup()
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
	dir, err := os.MkdirTemp(utils.RootPath(), ".buildadmin-crud-quarantine-")
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
		clean = filepath.Join(utils.RootPath(), clean)
	}
	clean, _ = filepath.Abs(clean)
	root := filepath.Clean(utils.RootPath())
	return filepath.Base(clean) == "provider.go" ||
		clean == filepath.Join(root, "router", "registrar_set.go") ||
		clean == filepath.Join(root, "cmd", "app", "wire.go") ||
		clean == filepath.Join(root, "cmd", "app", "wire_gen.go")
}

func atomicRestore(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".crud-restore-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
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
			return fmt.Errorf("replace %s: %w; rollback failed: %v", path, err, rollbackErr)
		}
		return err
	}
	return os.Remove(rollbackName)
}
