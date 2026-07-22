package validate

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type FlexInt32 int32
type FlexInt64 int64
type FlexFloat64 float64

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
