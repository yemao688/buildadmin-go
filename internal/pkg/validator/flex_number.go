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
