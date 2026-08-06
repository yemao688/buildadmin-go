package crud_helper

import (
	"strings"
	"testing"

	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/pkg/data_scope"

	"github.com/stretchr/testify/require"
)

// F1: 生成/删除流程的 AtomicRoute 注册注销键必须与 registrar 声明的
// CRUDCapabilities 键同形（小写斜杠形态，如 country/language），否则
// 生成后未重启时新模块写请求查不到能力，删除时也注销不掉。
func TestAtomicRouteRegistrationKeysMatchCapabilityShape(t *testing.T) {
	cases := []struct {
		tableName string
		// 期望能力键（与 admin/router capabilityRoute 同形：小写 + 点号转斜杠）
		wantKey string
	}{
		{"country_language_content", "country/languagecontent"},
		{"ops_user_test_xxx", "ops/usertestxxx"},
		{"user_money_log", "user/moneylog"},
		{"banner", "banner"},
		{"user", "user"},
	}
	for _, tc := range cases {
		t.Run(tc.tableName, func(t *testing.T) {
			// 对齐真实生成流程：normalizeTableConfiguration 会把
			// GenerateRelativePath 默认置为表名。
			table := crudmodel.Table{Name: tc.tableName, GenerateRelativePath: tc.tableName}
			fields := []crudmodel.Field{{Name: "id", Type: "int", PrimaryKey: true}}
			getTableName := func(n string, full bool) string {
				if full {
					return "ba_" + n
				}
				return n
			}
			_, handlerData, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, &data_scope.Config{Mode: data_scope.ModeNone}, getTableName, proveAll)
			require.NoError(t, err)

			// 生成流程构造注册 path 用的键（write.go RegisterAtomicRoute 分支）
			regName := handlerData.RouteName
			if regName == "" {
				regName = lowerFirst(handlerData.ClassName)
			}
			regKey := AtomicRouteCapabilityName(regName)

			// registrar 声明键（CRUDCapabilities → capabilityRoute 同形）
			capKey := strings.ToLower(strings.ReplaceAll(handlerData.RouteName, ".", "/"))

			require.Equal(t, tc.wantKey, capKey, "capability key derivation changed")
			require.Equal(t, capKey, regKey, "registration key must equal registrar capability key")
			require.NotContains(t, regKey, ".", "registration key must be slash-form")

			// 删除流程（atomicRoutesForName）注销 path 必须与生成注册 path 一致
			del := atomicRoutesForName(handlerData.RouteName)
			require.Equal(t, regKey+"/add", del[0].path)
			require.Equal(t, regKey+"/edit", del[1].path)
			require.Equal(t, regKey+"/del", del[2].path)
		})
	}
}

// F1 前缀碰撞用例：user 与 user_money_log 的能力键不得互相吞并
// （旧实现注销键按类名推导，userMoneyLog 与生成注册键不一致）。
func TestAtomicRouteKeysDoNotCollideAcrossPrefixes(t *testing.T) {
	keys := map[string]string{}
	for _, tableName := range []string{"user", "user_money_log", "user_order", "user_order_item"} {
		table := crudmodel.Table{Name: tableName, GenerateRelativePath: tableName}
		fields := []crudmodel.Field{{Name: "id", Type: "int", PrimaryKey: true}}
		getTableName := func(n string, full bool) string {
			if full {
				return "ba_" + n
			}
			return n
		}
		_, handlerData, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, &data_scope.Config{Mode: data_scope.ModeNone}, getTableName, proveAll)
		require.NoError(t, err)
		key := AtomicRouteCapabilityName(handlerData.RouteName)
		if previous, exists := keys[key]; exists {
			t.Fatalf("capability key %q collides between %q and %q", key, previous, tableName)
		}
		keys[key] = tableName
	}
	require.NotEqual(t, "user", keys["user/moneylog"], "user_money_log must not collapse into user")
}
