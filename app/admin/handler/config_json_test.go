package handler

import (
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestDecodeConfigJSON(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantCount int
		wantErr   string
	}{
		{name: "empty", value: " \n\t", wantCount: 0},
		{name: "valid", value: `[{"key":"basics","value":"Basics"}]`, wantCount: 1},
		{name: "invalid", value: "{", wantErr: "config_quick_entrance"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := decodeConfigJSON("config_quick_entrance", tt.value)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err=%v", err)
				}
				return
			}
			if err != nil || len(items) != tt.wantCount {
				t.Fatalf("items=%#v err=%v", items, err)
			}
		})
	}
}

func TestConfigJSONRecordNotFoundIsEmpty(t *testing.T) {
	items, err := decodeConfigValue("config_quick_entrance", "ignored", gorm.ErrRecordNotFound)
	if err != nil || len(items) != 0 {
		t.Fatalf("items=%#v err=%v", items, err)
	}
	_, err = decodeConfigValue("config_group", "", errors.New("database unavailable"))
	if err == nil || !strings.Contains(err.Error(), "read config_group") {
		t.Fatalf("err=%v", err)
	}
}
