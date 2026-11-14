package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	adminmodel "buildadmin-go/internal/admin/repository"
	siteconfig "buildadmin-go/internal/common/siteconfig"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// 变量名唯一性预检归 ConfigService（repo 只保留并发 duplicate-key 兜底），
// 新增与编辑都必须拒绝重名，编辑还需排除自身。
func TestConfigServiceRejectsDuplicateNames(t *testing.T) {
	t.Parallel()
	dsn := fmt.Sprintf("file:config-service-duplicate-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
	})
	require.NoError(t, err)
	require.NoError(t, testutil.CreateSQLiteConfigTable(db, "ba_config"))
	repo := adminmodel.NewConfigRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}, nil)
	svc := NewConfigService(repo)
	ctx := context.Background()

	require.NoError(t, svc.Add(ctx, ConfigParams{Name: "upload_mode"}))
	require.ErrorContains(t, svc.Add(ctx, ConfigParams{Name: "upload_mode"}), "config name already exists")

	require.NoError(t, svc.Add(ctx, ConfigParams{Name: "upload_bucket"}))
	var second siteconfig.Config
	require.NoError(t, db.Where("name = ?", "upload_bucket").First(&second).Error)

	// 改成已存在的名字被拒绝。
	err = svc.Edit(ctx, second.ID, ConfigParams{Name: "upload_mode"})
	require.ErrorContains(t, err, "config name already exists")

	// 保持自身名字的编辑不受预检影响（excludeID 生效）。
	require.NoError(t, svc.Edit(ctx, second.ID, ConfigParams{Name: "upload_bucket", Title: "Bucket"}))
	var renamed siteconfig.Config
	require.NoError(t, db.First(&renamed, second.ID).Error)
	require.Equal(t, "Bucket", renamed.Title)
}

type configJSONItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func TestUpdateConfigValueSkipsUnchangedValue(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
		if value := (&siteconfig.Config{}).SetValueAttr(params[name], "string"); value != expectedValue {
			t.Fatalf("%s=%q, want %q", name, value, expectedValue)
		}
	}

	newValue := (&siteconfig.Config{}).SetValueAttr(params["config_group"], "array")
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
