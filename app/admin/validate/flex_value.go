package validate

import (
	"bytes"
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
type FlexClock time.Time
type FlexUnixTime int64

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

func (v *FlexDate) UnmarshalJSON(data []byte) error {
	value, err := parseFlexTime(data, "2006-01-02")
	if err != nil {
		return err
	}
	*v = FlexDate(value)
	return nil
}

func (v *FlexClock) UnmarshalJSON(data []byte) error {
	value, err := parseFlexTime(data, "15:04:05")
	if err != nil {
		return err
	}
	*v = FlexClock(value)
	return nil
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
