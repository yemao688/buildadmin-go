package crud_helper

import (
	"fmt"
	crudmodel "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/utils"
	"net/url"
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
	"geometry": true, "geometrycollection": true, "linestring": true, "multilinestring": true,
	"multipoint": true, "multipolygon": true, "point": true, "polygon": true,
}

var parameterizedTypeRE = regexp.MustCompile(`^([a-z]+)\(([0-9]+)(,[0-9]+)?\)$`)
var enumTypeRE = regexp.MustCompile(`^(enum|set)\((?:'[^'\\\x00]*(?:''[^'\\\x00]*)*')(?:,(?:'[^'\\\x00]*(?:''[^'\\\x00]*)*'))*\)$`)
var windowsDrivePathRE = regexp.MustCompile(`^[A-Za-z]:`)

// ValidateGenerationInput is the shared validation boundary for all DDL input.
func ValidateGenerationInput(table crudmodel.Table, fields []crudmodel.Field) error {
	if err := normalizeTableConfiguration(&table); err != nil {
		return err
	}
	if err := data_scope.ValidateIdentifier(table.Name); err != nil {
		return fmt.Errorf("invalid table name: %w", err)
	}
	if err := validateSQLString(table.Comment); err != nil {
		return fmt.Errorf("invalid table comment: %w", err)
	}
	for _, field := range table.QuickSearchField {
		if strings.Contains(field, ".") {
			return fmt.Errorf("quickSearchField %q requires JOIN support for relation search; dotted relation quick search is deferred", field)
		}
	}
	for _, customPath := range []string{table.ModelFile, table.ControllerFile, table.WebViewsDir, table.ValidateFile} {
		if customPath != "" {
			if err := validateRelativePathInput(customPath); err != nil {
				return err
			}
		}
	}
	if _, err := ParseEntityNameData(table.Name, table.ModelFile); err != nil {
		return err
	}
	if _, err := ParseRepositoryNameData(table.Name, table.ModelFile); err != nil {
		return err
	}
	if _, err := ParseDTONameData(table.Name, table.ModelFile); err != nil {
		return err
	}
	if _, err := ParseHandlerNameData(table.Name, table.ControllerFile); err != nil {
		return err
	}
	if _, err := ParseRegistrarNameData(table.Name, table.ControllerFile); err != nil {
		return err
	}
	for i := range fields {
		normalizeFieldConfiguration(&fields[i])
		field := fields[i]
		if err := ValidateField(field); err != nil {
			return err
		}
		if err := validateRelationField(field); err != nil {
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

func validateRelationField(field crudmodel.Field) error {
	if field.Form.RemoteModel != "" && field.Form.RemoteTable == "" {
		return fmt.Errorf("remote model for field %q requires remote table", field.Name)
	}
	if field.Form.RemoteTable != "" {
		if err := data_scope.ValidateIdentifier(field.Form.RemoteTable); err != nil {
			return fmt.Errorf("invalid remote table for field %q: %w", field.Name, err)
		}
	}
	if field.Form.RemotePk != "" {
		parts := strings.Split(field.Form.RemotePk, ".")
		if len(parts) > 2 {
			return fmt.Errorf("invalid remote primary key for field %q: expected one identifier or owner.field", field.Name)
		}
		for _, part := range parts {
			if err := data_scope.ValidateIdentifier(part); err != nil {
				return fmt.Errorf("invalid remote primary key for field %q: %w", field.Name, err)
			}
		}
	}
	if field.Form.RemotePrimaryTableAlias != "" && !frontendIdentifierRE.MatchString(field.Form.RemotePrimaryTableAlias) {
		return fmt.Errorf("invalid remote primary table alias %q", field.Form.RemotePrimaryTableAlias)
	}
	if field.Form.RelationFields != "" {
		for _, name := range strings.Split(field.Form.RelationFields, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if err := data_scope.ValidateIdentifier(name); err != nil {
				return fmt.Errorf("invalid relation field %q for field %q: %w", name, field.Name, err)
			}
		}
	}
	if field.Form.RemoteModel != "" && field.Form.RemoteTable != "" {
		parsed, err := ParseEntityNameData(field.Form.RemoteTable, field.Form.RemoteModel)
		if err != nil {
			return fmt.Errorf("invalid remote model for field %q: %w", field.Name, err)
		}
		if err := ValidateGeneratedAbsolutePath(parsed.ParseFile, "internal/model"); err != nil {
			return fmt.Errorf("remote model for field %q escapes model roots: %w", field.Name, err)
		}
	}
	return nil
}

func validatePrimaryKeyTypes(fields []crudmodel.Field) error {
	for _, field := range fields {
		if !field.PrimaryKey {
			continue
		}
		if _, err := primaryKeyGoType(field); err != nil {
			return err
		}
		if field.AutoIncrement && !field.Unsigned {
			return fmt.Errorf("auto-increment primary key field %q must declare unsigned: true", field.Name)
		}
	}
	return nil
}

func ValidateField(field crudmodel.Field) error {
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
	if err := validateDefaultType(field); err != nil {
		return err
	}
	if err := validateSQLString(field.Comment); err != nil {
		return fmt.Errorf("invalid comment for field %q: %w", field.Name, err)
	}
	if err := validateFrontendField(field); err != nil {
		return err
	}
	return nil
}

var frontendIdentifierRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var frontendPathRE = regexp.MustCompile(`^[A-Za-z0-9_./\\-]+$`)
var validatorRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*(\([^\r\n()'";]*\))?$`)

// Remote is an object-literal fragment in the established designer format;
// keep it data-only because it cannot be JSON encoded without changing format.
var frontendObjectRE = regexp.MustCompile(`^[A-Za-z0-9_$\s:'".,/{}\[\]-]+$`)

func validateFrontendField(field crudmodel.Field) error {
	if field.Form.RemoteField != "" && !frontendIdentifierRE.MatchString(field.Form.RemoteField) {
		return fmt.Errorf("invalid remote field %q", field.Form.RemoteField)
	}
	if field.Form.RemoteController != "" {
		if strings.Contains(field.Form.RemoteController, "..") || !frontendPathRE.MatchString(field.Form.RemoteController) {
			return fmt.Errorf("invalid remote controller %q", field.Form.RemoteController)
		}
	}
	if err := validateFrontendURL(field.Form.RemoteUrl); err != nil {
		return fmt.Errorf("invalid remote URL for field %q: %w", field.Name, err)
	}
	for _, rule := range field.Form.Validator {
		if !validatorRE.MatchString(rule) {
			return fmt.Errorf("invalid validator %q for field %q", rule, field.Name)
		}
	}
	if err := validateFrontendMessage(field.Form.ValidatorMsg); err != nil {
		return fmt.Errorf("field %q: %w", field.Name, err)
	}
	for label, value := range map[string]string{
		"table label": field.Table.Label, "table operator": field.Table.Operator,
		"table render": field.Table.Render, "table sortable": field.Table.Sortable,
		"table show": field.Table.Show, "table search render": field.Table.ComSearchRender,
		"table time format": field.Table.TimeFormat,
	} {
		if err := validateFrontendText(value, label); err != nil {
			return fmt.Errorf("field %q: %w", field.Name, err)
		}
	}
	for key, value := range field.Table.ComSearchInputAttr {
		if err := validateFrontendText(key, "table search input attribute name"); err != nil {
			return fmt.Errorf("field %q: %w", field.Name, err)
		}
		if err := validateSearchInputAttrValue(value); err != nil {
			return fmt.Errorf("field %q: %w", field.Name, err)
		}
	}
	if err := validateFrontendText(field.Table.Remote, "table remote config"); err != nil {
		return fmt.Errorf("field %q: %w", field.Name, err)
	}
	if field.Table.Remote != "" && !frontendObjectRE.MatchString(field.Table.Remote) {
		return fmt.Errorf("field %q: invalid table remote config", field.Name)
	}
	return nil
}

func validateFrontendText(value, label string) error {
	if strings.IndexByte(value, 0) >= 0 || strings.ContainsAny(value, "\r\n`") {
		return fmt.Errorf("invalid %s", label)
	}
	return nil
}

func validateSearchInputAttrValue(value any) error {
	switch value := value.(type) {
	case string:
		return validateFrontendText(value, "table search input attribute")
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return nil
	case map[string]any:
		for key, nested := range value {
			if err := validateFrontendText(key, "table search input attribute name"); err != nil {
				return err
			}
			if err := validateSearchInputAttrValue(nested); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("invalid table search input attribute value")
	}
}

func validateFrontendMessage(value string) error {
	if strings.IndexByte(value, 0) >= 0 || strings.ContainsAny(value, "`") {
		return fmt.Errorf("invalid validator message")
	}
	return nil
}

func validateFrontendURL(value string) error {
	if value == "" || strings.ContainsAny(value, "\r\n'\"`") {
		if value == "" {
			return nil
		}
		return fmt.Errorf("URL contains forbidden characters")
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") {
		return fmt.Errorf("URL scheme is not allowed")
	}
	u, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("must be a site-relative path or an http(s) URL")
	}
	if u.Scheme == "" && u.Host == "" && u.Path != "" {
		return nil
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("must be a site-relative path or an http(s) URL")
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
	_, err := normalizeLogicalPath(value)
	return err
}

func normalizeLogicalPath(value string) (string, error) {
	if value == "" || strings.IndexByte(value, 0) >= 0 || filepath.IsAbs(filepath.FromSlash(value)) || windowsDrivePathRE.MatchString(value) {
		return "", fmt.Errorf("path must be relative")
	}
	value = strings.TrimSuffix(value, ".go")
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.ReplaceAll(value, ".", "/")
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("path contains an empty or invalid segment")
		}
	}
	return strings.Join(parts, "/"), nil
}

func validateDefaultType(field crudmodel.Field) error {
	switch field.DefaultType {
	case "":
		return nil
	case "INPUT", "NONE", "NULL", "EMPTY STRING":
		return nil
	default:
		return fmt.Errorf("invalid defaultType %q for field %q", field.DefaultType, field.Name)
	}
}

func normalizeFieldConfiguration(field *crudmodel.Field) {
	if field.DefaultType == "NULL" {
		field.Null = true
	}
}

func normalizeTableConfiguration(table *crudmodel.Table) error {
	if table.DatabaseConnection == "" {
		table.DatabaseConnection = "mysql"
	}
	if table.DatabaseConnection != "mysql" {
		return fmt.Errorf("unknown database connection %q; only \"mysql\" is available", table.DatabaseConnection)
	}
	// isCommonModel 已弃用：新语义下实体一律输出到共享记录层
	// internal/model（"全部共享"），历史 common model 模块仍可按 manifest 清理。
	if table.IsCommonModel != 0 {
		return fmt.Errorf("isCommonModel 已弃用：新语义下实体一律输出到 internal/model（全部共享）；历史 common model 模块仍可通过 crud:delete 清理")
	}
	if table.GenerateRelativePath == "" {
		table.GenerateRelativePath = table.Name
	}
	relative, err := normalizeLogicalPath(table.GenerateRelativePath)
	if err != nil {
		return fmt.Errorf("invalid generateRelativePath: %w", err)
	}
	table.GenerateRelativePath = relative
	if table.ModelFile == "" {
		table.ModelFile = "internal/model/" + relative + ".go"
	}
	if table.ControllerFile == "" {
		table.ControllerFile = "internal/admin/handler/" + relative + ".go"
	}
	if table.WebViewsDir == "" {
		table.WebViewsDir = "web/src/views/backend/" + relative
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
	return fmt.Errorf("path is outside permitted roots: %s", candidate)
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
	// 新语义下实体一律进入共享记录层 internal/model（common 参数保留以兼容调用方）。
	return ValidatePathUnderRoots(path, "internal/model")
}

func ValidateHandlerPath(path string) error { return ValidatePathUnderRoots(path, "internal/admin/handler") }

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
