package middleware

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

func extractOwnerID(row map[string]any, column string) (int32, error) {
	value, ok := row[column]
	if !ok {
		return 0, fmt.Errorf("target owner column %s missing", column)
	}
	var owner int64
	switch v := value.(type) {
	case int:
		owner = int64(v)
	case int32:
		owner = int64(v)
	case int64:
		owner = v
	case uint:
		owner = int64(v)
	case uint32:
		owner = int64(v)
	case uint64:
		if v > uint64(^uint32(0)) {
			return 0, fmt.Errorf("target owner out of range")
		}
		owner = int64(v)
	case float64:
		owner = int64(v)
	default:
		return 0, fmt.Errorf("target owner column %s has unsupported type", column)
	}
	if owner <= 0 || owner > int64(^uint32(0)>>1) {
		return 0, fmt.Errorf("target owner missing")
	}
	return int32(owner), nil
}

func normalizePrimaryKeyValue(value any) (string, error) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return "", fmt.Errorf("empty primary key")
		}
		return v, nil
	case []byte:
		return normalizePrimaryKeyValue(string(v))
	case json.Number:
		return string(v), nil
	case int:
		return strconv.FormatInt(int64(v), 10), nil
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case float64:
		if v != float64(int64(v)) {
			return "", fmt.Errorf("non-integral primary key")
		}
		return strconv.FormatInt(int64(v), 10), nil
	default:
		return "", fmt.Errorf("unsupported primary key type %T", value)
	}
}

// normalizeAuditValue gives database driver values a stable representation;
// in particular []byte must be treated as text rather than formatted as a
// numeric byte slice.
func normalizeAuditValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case []byte:
		return string(v)
	case time.Time:
		return v.UTC().Format(time.RFC3339Nano)
	case json.Number:
		return v.String()
	case float32:
		return strconv.FormatFloat(float64(v), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
