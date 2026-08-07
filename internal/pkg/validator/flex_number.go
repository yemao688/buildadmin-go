package validator

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type FlexBool bool
type FlexYear string
type FlexInt32 int32
type FlexInt64 int64
type FlexFloat64 float64

func (v *FlexBool) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		*v = false
		return nil
	}
	if strings.HasPrefix(text, `"`) {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return fmt.Errorf("invalid bool value: %w", err)
		}
		text = strings.TrimSpace(value)
	}
	parsed, err := parseFlexBoolText(text)
	if err != nil {
		return err
	}
	*v = FlexBool(parsed)
	return nil
}

func (v FlexBool) MarshalJSON() ([]byte, error) {
	if v {
		return []byte("1"), nil
	}
	return []byte("0"), nil
}

func (v *FlexBool) Scan(value any) error {
	var parsed bool
	switch value := value.(type) {
	case nil:
		parsed = false
	case bool:
		parsed = value
	case int:
		var err error
		parsed, err = flexBoolInt64(int64(value))
		if err != nil {
			return err
		}
	case int8:
		var err error
		parsed, err = flexBoolInt64(int64(value))
		if err != nil {
			return err
		}
	case int16:
		var err error
		parsed, err = flexBoolInt64(int64(value))
		if err != nil {
			return err
		}
	case int32:
		var err error
		parsed, err = flexBoolInt64(int64(value))
		if err != nil {
			return err
		}
	case int64:
		var err error
		parsed, err = flexBoolInt64(value)
		if err != nil {
			return err
		}
	case uint:
		var err error
		parsed, err = flexBoolUint64(uint64(value))
		if err != nil {
			return err
		}
	case uint8:
		var err error
		parsed, err = flexBoolUint64(uint64(value))
		if err != nil {
			return err
		}
	case uint16:
		var err error
		parsed, err = flexBoolUint64(uint64(value))
		if err != nil {
			return err
		}
	case uint32:
		var err error
		parsed, err = flexBoolUint64(uint64(value))
		if err != nil {
			return err
		}
	case uint64:
		var err error
		parsed, err = flexBoolUint64(value)
		if err != nil {
			return err
		}
	case string:
		var err error
		parsed, err = parseFlexBoolText(strings.TrimSpace(value))
		if err != nil {
			return err
		}
	case []byte:
		var err error
		parsed, err = parseFlexBoolText(strings.TrimSpace(string(value)))
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid bool scan value %T", value)
	}
	*v = FlexBool(parsed)
	return nil
}

func (v FlexBool) Value() (driver.Value, error) {
	if v {
		return int64(1), nil
	}
	return int64(0), nil
}

func parseFlexBoolText(text string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "", "0", "false":
		return false, nil
	case "1", "true":
		return true, nil
	default:
		return false, fmt.Errorf("invalid bool value %q: expected true, false, 0, or 1", text)
	}
}

func flexBoolInt64(value int64) (bool, error) {
	if value == 0 {
		return false, nil
	}
	if value == 1 {
		return true, nil
	}
	return false, fmt.Errorf("invalid bool scan value %d: expected 0 or 1", value)
}

func flexBoolUint64(value uint64) (bool, error) {
	if value == 0 {
		return false, nil
	}
	if value == 1 {
		return true, nil
	}
	return false, fmt.Errorf("invalid bool scan value %d: expected 0 or 1", value)
}

func (v *FlexYear) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		*v = ""
		return nil
	}
	if strings.HasPrefix(text, `"`) {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return fmt.Errorf("invalid year value: %w", err)
		}
		text = strings.TrimSpace(value)
	}
	year, err := parseFlexYear(text)
	if err != nil {
		return err
	}
	*v = FlexYear(year)
	return nil
}

func (v FlexYear) MarshalJSON() ([]byte, error) {
	if v == "" {
		return []byte("null"), nil
	}
	year, err := parseFlexYear(string(v))
	if err != nil {
		return nil, err
	}
	return json.Marshal(year)
}

func (v *FlexYear) Scan(value any) error {
	var text string
	switch value := value.(type) {
	case nil:
		text = ""
	case int:
		text = strconv.Itoa(value)
	case int8:
		text = strconv.FormatInt(int64(value), 10)
	case int16:
		text = strconv.FormatInt(int64(value), 10)
	case int32:
		text = strconv.FormatInt(int64(value), 10)
	case int64:
		text = strconv.FormatInt(value, 10)
	case uint:
		text = strconv.FormatUint(uint64(value), 10)
	case uint8:
		text = strconv.FormatUint(uint64(value), 10)
	case uint16:
		text = strconv.FormatUint(uint64(value), 10)
	case uint32:
		text = strconv.FormatUint(uint64(value), 10)
	case uint64:
		text = strconv.FormatUint(value, 10)
	case string:
		text = strings.TrimSpace(value)
	case []byte:
		text = strings.TrimSpace(string(value))
	default:
		return fmt.Errorf("invalid year scan value %T", value)
	}
	year, err := parseFlexYear(text)
	if err != nil {
		return err
	}
	*v = FlexYear(year)
	return nil
}

func (v FlexYear) Value() (driver.Value, error) {
	if v == "" {
		return nil, nil
	}
	year, err := parseFlexYear(string(v))
	if err != nil {
		return nil, err
	}
	return year, nil
}

func parseFlexYear(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	if text == "0" || text == "0000" {
		return "0", nil
	}
	if len(text) != 4 {
		return "", fmt.Errorf("invalid year value %q: expected 0 or a four-digit year from 1901 to 2155", text)
	}
	for _, char := range text {
		if char < '0' || char > '9' {
			return "", fmt.Errorf("invalid year value %q: expected 0 or a four-digit year from 1901 to 2155", text)
		}
	}
	year, err := strconv.Atoi(text)
	if err != nil || year < 1901 || year > 2155 {
		return "", fmt.Errorf("invalid year value %q: expected 0 or a four-digit year from 1901 to 2155", text)
	}
	return text, nil
}

func (v *FlexInt32) UnmarshalJSON(data []byte) error {
	value, err := flexNumberText(data)
	if err != nil {
		return fmt.Errorf("invalid int32 value: %w", err)
	}
	if value == "" {
		*v = 0
		return nil
	}
	n, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid int32 value %q: %w", value, err)
	}
	*v = FlexInt32(n)
	return nil
}

func (v *FlexInt64) UnmarshalJSON(data []byte) error {
	value, err := flexNumberText(data)
	if err != nil {
		return fmt.Errorf("invalid int64 value: %w", err)
	}
	if value == "" {
		*v = 0
		return nil
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid int64 value %q: %w", value, err)
	}
	*v = FlexInt64(n)
	return nil
}

func (v *FlexFloat64) UnmarshalJSON(data []byte) error {
	value, err := flexNumberText(data)
	if err != nil {
		return fmt.Errorf("invalid float64 value: %w", err)
	}
	if value == "" {
		*v = 0
		return nil
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("invalid float64 value %q: %w", value, err)
	}
	*v = FlexFloat64(n)
	return nil
}

// FlexInt32Slice 是 []int32 的宽松 JSON 绑定形态：接受数字数组
// （[1,2]）、字符串数字数组（["1","2"]）或混合数组；空数组与 null 均合法，
// 元素逐一走与 FlexInt32 相同的宽松解析（null/空串元素 → 0），非法元素
// 报错并携带索引信息。用途：手写请求结构中整型数组字段的 PHP 弱类型兼容
// （如 admin_group 请求的 rules）。
type FlexInt32Slice []int32

func (v *FlexInt32Slice) UnmarshalJSON(data []byte) error {
	if strings.TrimSpace(string(data)) == "null" {
		*v = nil
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) == 0 {
		*v = FlexInt32Slice{}
		return nil
	}
	out := make(FlexInt32Slice, len(raw))
	for i, item := range raw {
		text, err := flexNumberText(item)
		if err != nil {
			return fmt.Errorf("invalid int32 slice element at index %d: %w", i, err)
		}
		if text == "" {
			continue // null / 空串元素 → 0
		}
		n, err := strconv.ParseInt(text, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid int32 slice element at index %d (%q): %w", i, text, err)
		}
		out[i] = int32(n)
	}
	*v = out
	return nil
}

// FlexInt64Slice 是 []int64 的宽松 JSON 绑定形态：与 FlexInt32Slice 完全
// 同构（数字数组、字符串数字数组或混合数组；空数组与 null 均合法，null/
// 空串元素 → 0，非法元素报错并携带索引信息），仅元素位宽为 64。
// 用途：手写请求结构中 int64 整型数组字段的 PHP 弱类型兼容（与 32 位版
// 对称，供 id 类大整数数组字段使用）。
type FlexInt64Slice []int64

func (v *FlexInt64Slice) UnmarshalJSON(data []byte) error {
	if strings.TrimSpace(string(data)) == "null" {
		*v = nil
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) == 0 {
		*v = FlexInt64Slice{}
		return nil
	}
	out := make(FlexInt64Slice, len(raw))
	for i, item := range raw {
		text, err := flexNumberText(item)
		if err != nil {
			return fmt.Errorf("invalid int64 slice element at index %d: %w", i, err)
		}
		if text == "" {
			continue // null / 空串元素 → 0
		}
		n, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid int64 slice element at index %d (%q): %w", i, text, err)
		}
		out[i] = n
	}
	*v = out
	return nil
}

// FlexInt32Map 是 map[int32]int 的宽松 JSON 绑定形态。JSON 对象键恒为
// 字符串，严格 Go 解码 map[int32]int 时键字符串转 int32 会失败；本类型对
// 键与值都做宽松解析：键接受十进制数字串（"1"/1 形态），值接受数字或
// 字符串数字（null/空串 → 0），非法键/值报错并携带键信息。用途：手写请求
// 结构中 int32→int 映射字段的兼容（如 CRUD 上传完成的 syncIds）。
type FlexInt32Map map[int32]int

func (v *FlexInt32Map) UnmarshalJSON(data []byte) error {
	if strings.TrimSpace(string(data)) == "null" {
		*v = nil
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if *v == nil {
		*v = FlexInt32Map{}
	}
	for key, item := range raw {
		k, err := strconv.ParseInt(key, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid int32 map key %q: %w", key, err)
		}
		text, err := flexNumberText(item)
		if err != nil {
			return fmt.Errorf("invalid int32 map value for key %q: %w", key, err)
		}
		if text == "" {
			(*v)[int32(k)] = 0
			continue
		}
		n, err := strconv.ParseInt(text, 10, strconv.IntSize)
		if err != nil {
			return fmt.Errorf("invalid int32 map value %q for key %q: %w", text, key, err)
		}
		(*v)[int32(k)] = int(n)
	}
	return nil
}

func flexNumberText(data []byte) (string, error) {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		return "", nil
	}
	if text == "true" {
		return "1", nil
	}
	if text == "false" {
		return "0", nil
	}
	if strings.HasPrefix(text, `"`) {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return strings.TrimSpace(value), nil
	}
	if !json.Valid(data) {
		return "", fmt.Errorf("expected JSON number or numeric string")
	}
	return text, nil
}
