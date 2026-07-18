package crud_helper

import (
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var allowedSQLTypes = map[string]bool{
	"bigint": true, "int": true, "integer": true, "mediumint": true, "smallint": true, "tinyint": true,
	"decimal": true, "double": true, "float": true, "real": true,
	"char": true, "varchar": true, "text": true, "tinytext": true, "mediumtext": true, "longtext": true,
	"binary": true, "varbinary": true, "blob": true, "tinyblob": true, "mediumblob": true, "longblob": true,
	"date": true, "datetime": true, "timestamp": true, "time": true, "year": true,
	"json": true, "bool": true, "boolean": true,
}

var parameterizedTypeRE = regexp.MustCompile(`^([a-z]+)\(([0-9]+)(,[0-9]+)?\)$`)
var enumTypeRE = regexp.MustCompile(`^(enum|set)\((?:'[^'\\\x00]*(?:''[^'\\\x00]*)*')(?:,(?:'[^'\\\x00]*(?:''[^'\\\x00]*)*'))*\)$`)

// ValidateGenerationInput is the shared validation boundary for all DDL input.
func ValidateGenerationInput(table model.Table, fields []model.Field) error {
	if err := data_scope.ValidateIdentifier(table.Name); err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	if err := validateSQLString(table.Comment); err != nil {
		return fmt.Errorf("invalid table comment: %w", err)
	}
	for _, customPath := range []string{table.ModelFile, table.ControllerFile, table.WebViewsDir, table.ValidateFile} {
		if customPath != "" {
			if err := validateRelativePathInput(customPath); err != nil {
				return err
			}
		}
	}
	module := "admin"
	if table.IsCommonModel != 0 {
		module = "common"
	}
	if _, err := ParseNameData(module, table.Name, "model", table.ModelFile); err != nil {
		return err
	}
	if _, err := ParseNameData("admin", table.Name, "handler", table.ControllerFile); err != nil {
		return err
	}
	for _, field := range fields {
		if err := ValidateField(field); err != nil {
			return err
		}
	}
	if len(fields) == 0 {
		return fmt.Errorf("at least one field is required")
	}
	primaryKeys := 0
	names := map[string]string{}
	for _, field := range fields {
		key := strings.ToLower(field.Name)
		if previous, exists := names[key]; exists {
			return fmt.Errorf("duplicate field name %q (conflicts with %q)", field.Name, previous)
		}
		names[key] = field.Name
		if field.PrimaryKey {
			primaryKeys++
		}
	}
	if primaryKeys != 1 {
		return fmt.Errorf("exactly one primary key field is required, got %d", primaryKeys)
	}
	if err := validatePrimaryKeyTypes(fields); err != nil {
		return err
	}
	for _, change := range table.DesignChange {
		for _, name := range []string{change.OldName, change.NewName, change.After} {
			if name != "" && name != "FIRST FIELD" {
				if err := data_scope.ValidateIdentifier(name); err != nil {
					return fmt.Errorf("invalid design-change identifier %q: %w", name, err)
				}
			}
		}
	}
	return nil
}

func validatePrimaryKeyTypes(fields []model.Field) error {
	for _, field := range fields {
		if !field.PrimaryKey {
			continue
		}
		if _, err := primaryKeyGoType(field); err != nil {
			return err
		}
	}
	return nil
}

func ValidateField(field model.Field) error {
	if err := data_scope.ValidateIdentifier(field.Name); err != nil {
		return fmt.Errorf("invalid field name %q: %w", field.Name, err)
	}
	if field.PrimaryKey {
		if _, err := primaryKeyGoType(field); err != nil {
			return err
		}
	}
	typeValue := field.DataType
	if typeValue == "" {
		typeValue = field.Type
	}
	if err := validateDataType(typeValue); err != nil {
		return fmt.Errorf("invalid data type for field %q: %w", field.Name, err)
	}
	if err := validateSQLString(field.Default); err != nil {
		return fmt.Errorf("invalid default for field %q: %w", field.Name, err)
	}
	if err := validateSQLString(field.Comment); err != nil {
		return fmt.Errorf("invalid comment for field %q: %w", field.Name, err)
	}
	return nil
}

func validateDataType(value string) error {
	if value == "" {
		return nil
	}
	v := strings.ToLower(strings.TrimSpace(value))
	if allowedSQLTypes[v] || enumTypeRE.MatchString(v) {
		return nil
	}
	if match := parameterizedTypeRE.FindStringSubmatch(v); match != nil && allowedSQLTypes[match[1]] {
		return nil
	}
	return fmt.Errorf("unsupported SQL type %q", value)
}

func validateSQLString(value string) error {
	if strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("contains NUL byte")
	}
	return nil
}

func escapeSQLString(value string) string { return strings.ReplaceAll(value, "'", "''") }

// ValidatePathUnderRoots rejects absolute paths, traversal, and symlinked
// paths that resolve outside the supplied repository roots.
func ValidatePathUnderRoots(candidate string, roots ...string) error {
	if candidate == "" || filepath.IsAbs(candidate) {
		return fmt.Errorf("path must be relative")
	}
	clean := filepath.Clean(filepath.FromSlash(candidate))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes its root")
	}
	absCandidate := filepath.Join(utils.RootPath(), clean)
	return validateAbsolutePathUnderRoots(absCandidate, roots...)
}

func validateRelativePathInput(value string) error {
	if value == "" || filepath.IsAbs(filepath.FromSlash(value)) {
		return fmt.Errorf("path must be relative")
	}
	for _, part := range strings.FieldsFunc(filepath.ToSlash(value), func(r rune) bool { return r == '/' }) {
		if part == ".." {
			return fmt.Errorf("path traversal is not allowed")
		}
	}
	return nil
}

func validateAbsolutePathUnderRoots(candidate string, roots ...string) error {
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return err
	}
	for _, root := range roots {
		absRoot, err := filepath.Abs(filepath.Join(utils.RootPath(), root))
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(absRoot, absCandidate)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			if err := validateSymlinkContainment(absRoot, absCandidate); err == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("path is outside permitted roots")
}

func validateSymlinkContainment(root, candidate string) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	resolvedCandidate, err := resolveExistingPath(candidate)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedCandidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("symlink escapes root")
	}
	return nil
}

func resolveExistingPath(path string) (string, error) {
	current := path
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", err
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func ValidateModelPath(path string, common bool) error {
	root := "app/admin/model"
	if common {
		root = "app/common/model"
	}
	return ValidatePathUnderRoots(path, root)
}

func ValidateHandlerPath(path string) error { return ValidatePathUnderRoots(path, "app/admin/handler") }

func ValidateWebPath(path string, lang bool) error {
	root := "web/src/views"
	if lang {
		root = "web/src/lang"
	}
	return ValidatePathUnderRoots(path, root)
}

func ValidateGeneratedAbsolutePath(path string, roots ...string) error {
	return validateAbsolutePathUnderRoots(path, roots...)
}

func ValidateIndexName(name string) error { return data_scope.ValidateIdentifier(name) }

func formatDefault(value string) string {
	return "DEFAULT '" + escapeSQLString(value) + "'"
}

func formatComment(value string) string { return "COMMENT '" + escapeSQLString(value) + "'" }
