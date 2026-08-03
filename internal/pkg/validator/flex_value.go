package validator

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type CommaJoined string
type KeyValueArray string
type FlexDateTime time.Time
type FlexDate time.Time
type FlexClock string
type FlexUnixTime int64
type FlexFormattedUnixTime int64

func (v CommaJoined) MarshalJSON() ([]byte, error) {
	if v == "" {
		return []byte("[]"), nil
	}
	return json.Marshal(strings.Split(string(v), ","))
}

func (v *CommaJoined) Scan(value any) error {
	text, err := scanString(value)
	if err != nil {
		return err
	}
	*v = CommaJoined(text)
	return nil
}

func (v CommaJoined) Value() (driver.Value, error) {
	return string(v), nil
}

func (v *CommaJoined) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		*v = ""
		return nil
	}
	if strings.HasPrefix(text, `"`) {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*v = CommaJoined(value)
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("expected JSON string or scalar array: %w", err)
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		value, err := commaScalar(item)
		if err != nil {
			return err
		}
		values = append(values, value)
	}
	*v = CommaJoined(strings.Join(values, ","))
	return nil
}

func commaScalar(data []byte) (string, error) {
	text := strings.TrimSpace(string(data))
	if text == "null" {
		return "", fmt.Errorf("null array element is not allowed")
	}
	if strings.HasPrefix(text, `"`) {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return "", err
		}
		return value, nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return "", err
	}
	switch value := value.(type) {
	case json.Number:
		return value.String(), nil
	case bool:
		return strconv.FormatBool(value), nil
	default:
		return "", fmt.Errorf("array element must be a string, number, or boolean")
	}
}

type keyValueItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (v KeyValueArray) MarshalJSON() ([]byte, error) {
	if v == "" {
		return []byte("[]"), nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal([]byte(v), &items); err != nil {
		return nil, fmt.Errorf("invalid key/value array: %w", err)
	}
	if items == nil {
		return nil, fmt.Errorf("invalid key/value array: expected JSON array")
	}
	return json.Marshal(items)
}

func (v *KeyValueArray) Scan(value any) error {
	text, err := scanString(value)
	if err != nil {
		return err
	}
	*v = KeyValueArray(text)
	return nil
}

func (v KeyValueArray) Value() (driver.Value, error) {
	return string(v), nil
}

func (v *KeyValueArray) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		*v = ""
		return nil
	}
	if strings.HasPrefix(text, `"`) {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		if value == "" {
			*v = ""
			return nil
		}
		data = []byte(value)
	}
	var rawItems []json.RawMessage
	if err := json.Unmarshal(data, &rawItems); err != nil {
		return fmt.Errorf("expected key/value object array: %w", err)
	}
	items := make([]keyValueItem, 0, len(rawItems))
	for _, raw := range rawItems {
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil || object == nil {
			return fmt.Errorf("key/value array element must be an object")
		}
		key, ok := object["key"]
		var keyValue, valueValue string
		if !ok || json.Unmarshal(key, &keyValue) != nil {
			return fmt.Errorf("key must be a string")
		}
		value, ok := object["value"]
		if !ok || json.Unmarshal(value, &valueValue) != nil {
			return fmt.Errorf("value must be a string")
		}
		items = append(items, keyValueItem{Key: keyValue, Value: valueValue})
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return err
	}
	*v = KeyValueArray(encoded)
	return nil
}

func parseFlexTime(data []byte, layouts ...string) (time.Time, error) {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		return time.Time{}, nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return time.Time{}, fmt.Errorf("expected JSON string: %w", err)
	}
	if value == "" {
		return time.Time{}, nil
	}
	for _, layout := range layouts {
		var parsed time.Time
		var err error
		if layout == time.RFC3339 {
			parsed, err = time.Parse(layout, value)
		} else {
			parsed, err = time.ParseInLocation(layout, value, time.Local)
		}
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time value %q", value)
}

func (v *FlexDateTime) UnmarshalJSON(data []byte) error {
	value, err := parseFlexTime(data, "2006-01-02 15:04:05", time.RFC3339)
	if err != nil {
		return err
	}
	*v = FlexDateTime(value)
	return nil
}

func (v FlexDateTime) MarshalJSON() ([]byte, error) {
	value := time.Time(v)
	if value.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(value.In(time.Local).Format("2006-01-02 15:04:05"))
}

func (v *FlexDateTime) Scan(value any) error {
	parsed, err := scanFlexDateTime(value, "2006-01-02 15:04:05", time.RFC3339)
	if err != nil {
		return err
	}
	*v = FlexDateTime(parsed)
	return nil
}

func (v FlexDateTime) Value() (driver.Value, error) {
	value := time.Time(v)
	if value.IsZero() {
		return nil, nil
	}
	return value, nil
}

func (v *FlexDate) UnmarshalJSON(data []byte) error {
	value, err := parseFlexTime(data, "2006-01-02")
	if err != nil {
		return err
	}
	*v = FlexDate(value)
	return nil
}

func (v FlexDate) MarshalJSON() ([]byte, error) {
	value := time.Time(v)
	if value.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(value.In(time.Local).Format("2006-01-02"))
}

func (v *FlexDate) Scan(value any) error {
	parsed, err := scanFlexDateTime(value, "2006-01-02")
	if err != nil {
		return err
	}
	*v = FlexDate(parsed)
	return nil
}

func (v FlexDate) Value() (driver.Value, error) {
	value := time.Time(v)
	if value.IsZero() {
		return nil, nil
	}
	return value, nil
}

func (v *FlexClock) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		*v = ""
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("expected JSON string: %w", err)
	}
	if value == "" {
		*v = ""
		return nil
	}
	if len(value) != len("15:04:05") || value[2] != ':' || value[5] != ':' ||
		!isASCIIDigit(value[0]) || !isASCIIDigit(value[1]) ||
		!isASCIIDigit(value[3]) || !isASCIIDigit(value[4]) ||
		!isASCIIDigit(value[6]) || !isASCIIDigit(value[7]) {
		return fmt.Errorf("invalid clock value %q", value)
	}
	parsed, err := time.Parse("15:04:05", value)
	if err != nil {
		return fmt.Errorf("invalid clock value %q: %w", value, err)
	}
	*v = FlexClock(parsed.Format("15:04:05"))
	return nil
}

func (v FlexClock) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(v))
}

func (v *FlexClock) Scan(value any) error {
	text, err := scanString(value)
	if err != nil {
		return err
	}
	if text == "" {
		*v = ""
		return nil
	}
	var parsed FlexClock
	if err := parsed.UnmarshalJSON([]byte(strconv.Quote(text))); err != nil {
		return err
	}
	*v = parsed
	return nil
}

func (v FlexClock) Value() (driver.Value, error) {
	return string(v), nil
}

func isASCIIDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

func (v *FlexUnixTime) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		*v = 0
		return nil
	}
	var value string
	if text[0] == '"' {
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
	} else {
		if text == "true" || text == "false" || strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
			return fmt.Errorf("invalid unix time value")
		}
		value = text
	}
	if value == "" {
		*v = 0
		return nil
	}
	if unix, err := strconv.ParseInt(value, 10, 64); err == nil {
		*v = FlexUnixTime(unix)
		return nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339} {
		var parsed time.Time
		var err error
		if layout == time.RFC3339 {
			parsed, err = time.Parse(layout, value)
		} else {
			parsed, err = time.ParseInLocation(layout, value, time.Local)
		}
		if err == nil {
			*v = FlexUnixTime(parsed.Unix())
			return nil
		}
	}
	return fmt.Errorf("invalid unix time value %q", value)
}

func (v FlexUnixTime) MarshalJSON() ([]byte, error) {
	if v == 0 {
		return []byte("null"), nil
	}
	return json.Marshal(int64(v))
}

func (v *FlexUnixTime) Scan(value any) error {
	switch value := value.(type) {
	case nil:
		*v = 0
		return nil
	case time.Time:
		*v = FlexUnixTime(value.Unix())
		return nil
	case int64:
		*v = FlexUnixTime(value)
		return nil
	case int32:
		*v = FlexUnixTime(value)
		return nil
	case int:
		*v = FlexUnixTime(value)
		return nil
	case string, []byte:
		text, err := scanString(value)
		if err != nil {
			return err
		}
		if text == "" {
			*v = 0
			return nil
		}
		if unix, err := strconv.ParseInt(text, 10, 64); err == nil {
			*v = FlexUnixTime(unix)
			return nil
		}
		parsed, err := parseStringFlexTime(text, "2006-01-02 15:04:05", time.RFC3339)
		if err != nil {
			return err
		}
		*v = FlexUnixTime(parsed.Unix())
		return nil
	default:
		return fmt.Errorf("unsupported unix time scan value %T", value)
	}
}

func (v FlexUnixTime) Value() (driver.Value, error) {
	return int64(v), nil
}

func (v *FlexFormattedUnixTime) UnmarshalJSON(data []byte) error {
	var unix FlexUnixTime
	if err := unix.UnmarshalJSON(data); err != nil {
		return err
	}
	*v = FlexFormattedUnixTime(unix)
	return nil
}

func (v FlexFormattedUnixTime) MarshalJSON() ([]byte, error) {
	if v == 0 {
		return []byte("null"), nil
	}
	return json.Marshal(time.Unix(int64(v), 0).In(time.Local).Format("2006-01-02 15:04:05"))
}

func (v *FlexFormattedUnixTime) Scan(value any) error {
	var unix FlexUnixTime
	if err := unix.Scan(value); err != nil {
		return err
	}
	*v = FlexFormattedUnixTime(unix)
	return nil
}

func (v FlexFormattedUnixTime) Value() (driver.Value, error) {
	return int64(v), nil
}

func scanString(value any) (string, error) {
	switch value := value.(type) {
	case nil:
		return "", nil
	case string:
		return value, nil
	case []byte:
		return string(value), nil
	default:
		return "", fmt.Errorf("unsupported string scan value %T", value)
	}
}

func scanFlexDateTime(value any, layouts ...string) (time.Time, error) {
	if value == nil {
		return time.Time{}, nil
	}
	if parsed, ok := value.(time.Time); ok {
		return parsed, nil
	}
	text, err := scanString(value)
	if err != nil {
		return time.Time{}, err
	}
	if text == "" {
		return time.Time{}, nil
	}
	return parseStringFlexTime(text, layouts...)
}

func parseStringFlexTime(value string, layouts ...string) (time.Time, error) {
	for _, layout := range layouts {
		var parsed time.Time
		var err error
		if layout == time.RFC3339 {
			parsed, err = time.Parse(layout, value)
		} else {
			parsed, err = time.ParseInLocation(layout, value, time.Local)
		}
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time value %q", value)
}
