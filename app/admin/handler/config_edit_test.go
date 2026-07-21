package handler

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"go-build-admin/app/admin/model"
)

func TestUpdateConfigValueSkipsUnchangedValue(t *testing.T) {
	called := false
	err := updateConfigValue("same", "same", func() (int64, error) {
		called = true
		return 0, errors.New("no-op update should not run")
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if called {
		t.Fatal("update callback was called for an unchanged value")
	}
}

func TestUpdateConfigValueKeepsChangedValueChecks(t *testing.T) {
	called := false
	err := updateConfigValue("old", "new", func() (int64, error) {
		called = true
		return 0, nil
	})
	if !called {
		t.Fatal("update callback was not called for a changed value")
	}
	if err == nil {
		t.Fatal("expected rows affected mismatch error")
	}
}

func TestConfigEditArrayValueFromJSONRequestIsUpdated(t *testing.T) {
	requestJSON := `{"site_name":"Hotel","no_access_ip":"","time_zone":"Asia/Shanghai","version":"v1.0.0","record_number":"渝ICP备8888888号-1","config_group":[{"key":"basics","value":"Basics"},{"key":"mail","value":"Mail"},{"key":"config_quick_entrance","value":"Config Quick entrance"},{"key":"upload","value":"上传配置"}]}`
	params := map[string]interface{}{}
	if err := json.Unmarshal([]byte(requestJSON), &params); err != nil {
		t.Fatalf("decode request JSON: %v", err)
	}
	for name, expectedValue := range map[string]string{
		"site_name":     "Hotel",
		"no_access_ip":  "",
		"time_zone":     "Asia/Shanghai",
		"version":       "v1.0.0",
		"record_number": "渝ICP备8888888号-1",
	} {
		if value := (&model.Config{}).SetValueAttr(params[name], "string"); value != expectedValue {
			t.Fatalf("%s=%q, want %q", name, value, expectedValue)
		}
	}

	newValue := (&model.Config{}).SetValueAttr(params["config_group"], "array")
	var entries []configJSONItem
	if err := json.Unmarshal([]byte(newValue), &entries); err != nil {
		t.Fatalf("decode serialized config_group: %v", err)
	}
	expected := []configJSONItem{
		{Key: "basics", Value: "Basics"},
		{Key: "mail", Value: "Mail"},
		{Key: "config_quick_entrance", Value: "Config Quick entrance"},
		{Key: "upload", Value: "上传配置"},
	}
	if !reflect.DeepEqual(entries, expected) {
		t.Fatalf("entries=%#v, want %#v", entries, expected)
	}

	existingValue := `[{"key":"basics","value":"Basics"},{"key":"mail","value":"Mail"},{"key":"config_quick_entrance","value":"Config Quick entrance"},{"key":"upload","value":"Upload"}]`
	callbackValue := ""
	err := updateConfigValue(existingValue, newValue, func() (int64, error) {
		callbackValue = newValue
		return 1, nil
	})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if callbackValue != newValue {
		t.Fatalf("callbackValue=%q, want serialized new value %q", callbackValue, newValue)
	}
}
