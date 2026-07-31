package conf

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// DefaultsFileName is the tracked runtime configuration base filename.
const DefaultsFileName = "config.defaults.yaml"

// LoadLayeredConfig reads the tracked defaults and optionally merges a sparse
// runtime override file on top of them. The returned Viper watches whichever
// file was loaded last, so callers can attach the existing reload behavior.
func LoadLayeredConfig(defaultsPath, overridePath string) (*viper.Viper, []string, error) {
	defaults, err := readYAMLMap(defaultsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read defaults config %q: %w", defaultsPath, err)
	}

	v := viper.New()
	v.SetConfigFile(defaultsPath)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
		return nil, nil, fmt.Errorf("read defaults config %q: %w", defaultsPath, err)
	}

	if overridePath == "" {
		return v, nil, nil
	}

	overrides, err := readYAMLMap(overridePath)
	if err != nil {
		return nil, nil, fmt.Errorf("read config override %q: %w", overridePath, err)
	}

	deadKeys := FindDeadKeys(defaults, overrides)
	v.SetConfigFile(overridePath)
	v.SetConfigType("yaml")
	// MergeConfig recursively combines mappings, while scalar and list values
	// replace the value from the defaults. Lists are not concatenated.
	if err := v.MergeInConfig(); err != nil {
		return nil, nil, fmt.Errorf("merge config override %q: %w", overridePath, err)
	}

	return v, deadKeys, nil
}

// MergeConfigMaps returns a deep merge in which override values win. Mappings
// are merged recursively; slices are copied and replaced as a whole.
func MergeConfigMaps(base, override map[string]any) map[string]any {
	merged := cloneMap(base)
	mergeMap(merged, override)
	return merged
}

// FindDeadKeys returns override leaf paths that do not exist in the defaults
// tree. Empty mappings are treated as leaves so an empty unknown section is
// still reported.
func FindDeadKeys(defaults, overrides map[string]any) []string {
	defaultKeys := make(map[string]struct{})
	for _, key := range flattenKeys(defaults, "") {
		defaultKeys[key] = struct{}{}
	}

	dead := make([]string, 0)
	seen := make(map[string]struct{})
	for _, key := range flattenKeys(overrides, "") {
		if _, ok := defaultKeys[key]; ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		dead = append(dead, key)
	}
	sort.Strings(dead)
	return dead
}

// WriteConfigOverrides preserves existing user overrides and writes only the
// supplied installation values. The header documents why this file is sparse.
func WriteConfigOverrides(path string, overrides map[string]any) error {
	existing := map[string]any{}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := yaml.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("parse config override %q: %w", path, err)
		}
	case os.IsNotExist(err):
		// A direct installer request may create the override file itself.
	default:
		return err
	}
	if existing == nil {
		existing = map[string]any{}
	}

	data, err = yaml.Marshal(MergeConfigMaps(existing, overrides))
	if err != nil {
		return fmt.Errorf("marshal config override %q: %w", path, err)
	}
	content := []byte("# Runtime overrides; unspecified keys come from config.defaults.yaml.\n\n")
	content = append(content, data...)
	if err := os.WriteFile(path, content, 0600); err != nil {
		return fmt.Errorf("write config override %q: %w", path, err)
	}
	return nil
}

func readYAMLMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return map[string]any{}, nil
	}

	value, ok := normalizeMap(raw).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("top-level YAML value must be a mapping")
	}
	return value, nil
}

func normalizeMap(value any) any {
	switch value := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(value))
		for key, child := range value {
			result[key] = normalizeMap(child)
		}
		return result
	case map[any]any:
		result := make(map[string]any, len(value))
		for key, child := range value {
			result[fmt.Sprint(key)] = normalizeMap(child)
		}
		return result
	case []any:
		result := make([]any, len(value))
		for index, child := range value {
			result[index] = normalizeMap(child)
		}
		return result
	default:
		return value
	}
}

func flattenKeys(values map[string]any, prefix string) []string {
	keys := make([]string, 0)
	for key, value := range values {
		fullKey := strings.ToLower(key)
		if prefix != "" {
			fullKey = prefix + "." + fullKey
		}
		if child, ok := value.(map[string]any); ok {
			if len(child) == 0 {
				keys = append(keys, fullKey)
			} else {
				keys = append(keys, flattenKeys(child, fullKey)...)
			}
			continue
		}
		keys = append(keys, fullKey)
	}
	return keys
}

func cloneMap(values map[string]any) map[string]any {
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = cloneValue(value)
	}
	return cloned
}

func cloneValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		return cloneMap(value)
	case []any:
		cloned := make([]any, len(value))
		for index, child := range value {
			cloned[index] = cloneValue(child)
		}
		return cloned
	default:
		return value
	}
}

func mergeMap(base, override map[string]any) {
	for key, value := range override {
		baseValue, baseIsMap := base[key].(map[string]any)
		overrideValue, overrideIsMap := value.(map[string]any)
		if baseIsMap && overrideIsMap {
			mergeMap(baseValue, overrideValue)
			continue
		}
		base[key] = cloneValue(value)
	}
}
