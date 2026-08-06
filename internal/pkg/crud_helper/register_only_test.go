package crud_helper

import (
	"testing"

	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/pkg/data_scope"

	"github.com/stretchr/testify/require"
)

// TestRegisterOnlyRejectsNonProtectedTableBeforeDependencies 验证三分支：
// registerOnly + 非受保护表在依赖可用之前即被拒绝（nil db/cfg 也能给出
// 明确错误，与受保护表拒绝的先例一致）。
func TestRegisterOnlyRejectsNonProtectedTableBeforeDependencies(t *testing.T) {
	_, err := GenerateFromSpec(nil, nil, GenerateOptions{
		Table: crudmodel.Table{Name: "orders", RegisterOnly: true},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), `registerOnly is reserved for protected core tables`)
	require.Contains(t, err.Error(), `"orders" is not protected`)
}

// TestRegisterOnlyFunctionLevelRejectsNonProtected 验证 registerOnlyFromSpec
// 内部防御性复核（函数级调用同样 fail-closed；纯校验无需 db）。
func TestRegisterOnlyFunctionLevelRejectsNonProtected(t *testing.T) {
	_, err := registerOnlyFromSpec(GenerateOptions{
		Table: crudmodel.Table{Name: "orders", RegisterOnly: true},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "is not protected")
}

// TestRegisterOnlyRequiresDataScope 验证 registerOnly 表必须声明 reassignable
// 或 inheritFrom（否则登记无意义）。
func TestRegisterOnlyRequiresDataScope(t *testing.T) {
	_, err := registerOnlyFromSpec(GenerateOptions{
		Table: crudmodel.Table{Name: "user", RegisterOnly: true},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "must declare dataScope.reassignable (parent registration) or dataScope.inheritFrom (child registration)")
}

// TestRegisterOnlyParentRegistration 验证父表登记（reassignable）纯校验通过，
// 不写 crud_log、不生成文件（返回空结果）。
func TestRegisterOnlyParentRegistration(t *testing.T) {
	result, err := registerOnlyFromSpec(GenerateOptions{
		Table: crudmodel.Table{
			Name:         "user",
			RegisterOnly: true,
			Comment:      "会员表",
			DataScope:    &data_scope.Config{Mode: data_scope.ModeAuto, Reassignable: true},
		},
		Fields: []crudmodel.Field{
			{Name: "id", Type: "int", PrimaryKey: true},
			{Name: "admin_id", Type: "int"},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Empty(t, result.Files)
	require.Zero(t, result.LogID) // 不写 crud_log
}

// TestRegisterOnlyChildRequiresRegisteredParent 验证子表登记（inheritFrom）的
// fail-closed：父表 spec 未登记（仓库无 crud_specs/<parent>.yaml）时拒绝——
// 用不存在的父表名验证，不依赖仓库状态。
func TestRegisterOnlyChildRequiresRegisteredParent(t *testing.T) {
	_, err := registerOnlyFromSpec(GenerateOptions{
		Table: crudmodel.Table{
			Name:         "user_money_log",
			RegisterOnly: true,
			Comment:      "会员余额变动表",
			DataScope: &data_scope.Config{
				Mode:        data_scope.ModeAuto,
				InheritFrom: &data_scope.InheritRef{Table: "seller_user_not_registered", ByColumn: "user_id"},
			},
		},
		Fields: []crudmodel.Field{
			{Name: "id", Type: "int", PrimaryKey: true},
			{Name: "user_id", Type: "int"},
			{Name: "admin_id", Type: "int"},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "inheritFrom parent")
}

// TestRegisterOnlyDataScopeConfigCarries 验证 registerOnly 标记随 spec 解析
// 进入 Table（LoadSpec 解析形态），且与数据权限声明并存。
func TestRegisterOnlyDataScopeConfigCarries(t *testing.T) {
	cfg := &data_scope.Config{Mode: data_scope.ModeAuto, Reassignable: true}
	table := crudmodel.Table{Name: "user", RegisterOnly: true, DataScope: cfg}
	require.True(t, table.RegisterOnly)
	require.NotNil(t, table.DataScope)
	require.True(t, table.DataScope.Reassignable)
}
