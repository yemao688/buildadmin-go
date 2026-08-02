package crud_helper

import (
	"errors"
	"fmt"
	crudmodel "go-build-admin/internal/admin/model/crud"
	"go-build-admin/internal/utils"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidationWarning is a non-fatal issue found by crud:validate.
type ValidationWarning struct {
	SpecPath string
	Message  string
}

// ValidateSpec validates one CRUD spec without opening a database or running
// any generation code. It returns warnings separately so callers can keep a
// successful exit status when a spec is valid but needs attention.
func ValidateSpec(path string) ([]ValidationWarning, error) {
	opts, err := LoadSpec(path)
	if err != nil {
		return nil, err
	}

	metadata, err := loadSpecValidationMetadata(path)
	if err != nil {
		return nil, err
	}

	warnings := make([]ValidationWarning, 0)
	errorsFound := make([]error, 0)
	for index, field := range opts.Fields {
		if index >= len(metadata.Fields) {
			errorsFound = append(errorsFound, fmt.Errorf("field %q could not be read from the spec YAML", field.Name))
			continue
		}
		if err := validateDefaultPairing(field, metadata.Fields[index]); err != nil {
			errorsFound = append(errorsFound, err)
		}

		if field.Form.RemoteController != "" {
			exists, err := validateRemoteFile("remoteController", field.Name, field.Form.RemoteController)
			if err != nil {
				errorsFound = append(errorsFound, err)
			} else if exists && routeIndexURLForController(field.Form.RemoteController) == "" {
				warnings = append(warnings, ValidationWarning{
					SpecPath: path,
					Message:  fmt.Sprintf("field %q remoteController %q exists but no registered route constant was found; the select URL will be derived by convention and may return 404", field.Name, field.Form.RemoteController),
				})
			}
		}
		if field.Form.RemoteModel != "" {
			if _, err := validateRemoteFile("remoteModel", field.Name, field.Form.RemoteModel); err != nil {
				errorsFound = append(errorsFound, err)
			}
		}
	}

	if metadata.RelativePathSet && hasNonSnakeEntitySegment(opts.Table.GenerateRelativePath) {
		// D3 only reports a path whose entity segment contains uppercase letters.
		// A deeper path such as ops/user/test_xxx remains valid when every segment
		// is snake_case; deeper directories are an intentional supported layout.
		warnings = append(warnings, ValidationWarning{
			SpecPath: path,
			Message:  fmt.Sprintf("generateRelativePath %q uses an uppercase/camel-case entity segment; standardize it to the table name %q", opts.Table.GenerateRelativePath, opts.Table.Name),
		})
	}

	if len(errorsFound) > 0 {
		return warnings, errors.Join(errorsFound...)
	}
	return warnings, nil
}

// ValidateSpecs validates all supplied specs and aggregates errors so a
// pre-commit invocation reports every bad file in one pass.
func ValidateSpecs(paths []string) ([]ValidationWarning, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one spec path is required")
	}

	warnings := make([]ValidationWarning, 0)
	errorsFound := make([]error, 0)
	for _, path := range paths {
		pathWarnings, err := ValidateSpec(path)
		warnings = append(warnings, pathWarnings...)
		if err != nil {
			errorsFound = append(errorsFound, fmt.Errorf("spec %q: %w", path, err))
		}
	}
	if len(errorsFound) > 0 {
		return warnings, errors.Join(errorsFound...)
	}
	return warnings, nil
}

func validateRemoteFile(kind, fieldName, value string) (bool, error) {
	normalized, err := normalizeLogicalPath(value)
	if err != nil {
		return false, fmt.Errorf("field %q %s %q: %w", fieldName, kind, value, err)
	}
	logicalPath := normalized
	if strings.HasSuffix(strings.ReplaceAll(value, "\\", "/"), ".go") {
		logicalPath += ".go"
	}
	candidate := filepath.Join(utils.RootPath(), filepath.FromSlash(logicalPath))
	info, err := os.Stat(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("field %q %s %q does not exist", fieldName, kind, value)
		}
		return false, fmt.Errorf("field %q %s %q cannot be checked: %w", fieldName, kind, value, err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("field %q %s %q is a directory, not a file", fieldName, kind, value)
	}
	return true, nil
}

func hasNonSnakeEntitySegment(relativePath string) bool {
	normalized, err := normalizeLogicalPath(relativePath)
	if err != nil {
		return false
	}
	parts := strings.Split(normalized, "/")
	_, entity := splitLogicalNameParts(parts)
	return entity != "" && entity != strings.ToLower(entity)
}

type specValidationMetadata struct {
	RelativePathSet bool
	Fields          []fieldValidationMetadata
}

type fieldValidationMetadata struct {
	Default        string
	DefaultSet     bool
	DefaultTypeSet bool
}

func loadSpecValidationMetadata(path string) (specValidationMetadata, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return specValidationMetadata{}, fmt.Errorf("read spec %q: %w", path, err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return specValidationMetadata{}, fmt.Errorf("parse spec %q: %w", path, err)
	}
	root := documentRoot(&document)
	if root == nil || root.Kind != yaml.MappingNode {
		return specValidationMetadata{}, fmt.Errorf("spec %q must be a YAML mapping", path)
	}
	metadata := specValidationMetadata{}
	if mappingNodeValue(root, "generateRelativePath") != nil {
		metadata.RelativePathSet = true
	}
	fieldsNode := mappingNodeValue(root, "fields")
	if fieldsNode != nil && fieldsNode.Kind == yaml.AliasNode {
		fieldsNode = fieldsNode.Alias
	}
	if fieldsNode == nil || fieldsNode.Kind != yaml.SequenceNode {
		return metadata, nil
	}
	metadata.Fields = make([]fieldValidationMetadata, 0, len(fieldsNode.Content))
	for _, fieldNode := range fieldsNode.Content {
		fieldMetadata := fieldValidationMetadata{}
		if fieldNode.Kind == yaml.AliasNode {
			fieldNode = fieldNode.Alias
		}
		if fieldNode == nil || fieldNode.Kind != yaml.MappingNode {
			metadata.Fields = append(metadata.Fields, fieldMetadata)
			continue
		}
		if defaultNode := mappingNodeValue(fieldNode, "default"); defaultNode != nil {
			fieldMetadata.DefaultSet = true
			fieldMetadata.Default = yamlNodeString(defaultNode)
		}
		if mappingNodeValue(fieldNode, "defaultType") != nil {
			fieldMetadata.DefaultTypeSet = true
		}
		metadata.Fields = append(metadata.Fields, fieldMetadata)
	}
	return metadata, nil
}

func documentRoot(document *yaml.Node) *yaml.Node {
	if document == nil || len(document.Content) == 0 {
		return nil
	}
	root := document.Content[0]
	if root.Kind == yaml.AliasNode {
		return root.Alias
	}
	return root
}

func mappingNodeValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return node.Content[index+1]
		}
	}
	return nil
}

func yamlNodeString(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	if node.Tag == "!!null" {
		return "null"
	}
	return node.Value
}

func validateDefaultPairing(field crudmodel.Field, metadata fieldValidationMetadata) error {
	if !metadata.DefaultSet {
		return nil
	}

	value := strings.ToLower(strings.TrimSpace(metadata.Default))
	if value == "" && (!metadata.DefaultTypeSet || field.DefaultType != "INPUT") {
		return fmt.Errorf("field %q explicitly sets an empty default; set defaultType: INPUT to mean DEFAULT ''", field.Name)
	}
	if !metadata.DefaultTypeSet {
		return nil
	}

	switch field.DefaultType {
	case "INPUT":
		return nil
	case "NULL":
		if value != "" && value != "null" {
			return fmt.Errorf("field %q default %q does not match defaultType NULL", field.Name, metadata.Default)
		}
	case "EMPTY STRING":
		if value != "" && value != "empty string" {
			return fmt.Errorf("field %q default %q does not match defaultType EMPTY STRING", field.Name, metadata.Default)
		}
	case "NONE":
		if value != "" && value != "none" {
			return fmt.Errorf("field %q default %q does not match defaultType NONE", field.Name, metadata.Default)
		}
	}
	return nil
}
