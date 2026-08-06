package crud_helper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeTestSpec 向临时 specs 目录写入一个合法最小 spec（name + id 主键）。
func writeTestSpec(t *testing.T, specsDir, name, dataScopeYAML string) {
	t.Helper()
	spec := "name: " + name + "\ncomment: test\n" + dataScopeYAML +
		"fields:\n" +
		"  - name: id\n    type: bigint\n    unsigned: true\n    primaryKey: true\n    autoIncrement: true\n    null: false\n    comment: ID\n" +
		"  - name: admin_id\n    type: int\n    unsigned: true\n    null: false\n    comment: 管理员ID\n"
	if err := os.WriteFile(filepath.Join(specsDir, name+".yaml"), []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestValidateInheritParent 验证 inheritFrom 父表校验（specs 驱动）：
// 父表 spec 缺失或未声明 reassignable 时拒绝；硬契约（owner 列、主键）落实。
// repo 落点（internal/admin/repository/<table>.go 含锚点块）检查走可注入的
// repoRootOverride：隔离根下"仓库无该表 repo"恒成立，业务 fork 拥有
// seller_user.go 时测试也不失真。
func TestValidateInheritParent(t *testing.T) {
	specsDir := t.TempDir()
	// 隔离仓库状态：repo 落点检查指向临时根，不依赖真实仓库内容。
	repoRootOverride = t.TempDir()
	t.Cleanup(func() { repoRootOverride = "" })

	// 无父表 spec → 拒绝
	err := validateInheritParent(specsDir, "seller_user")
	require.Error(t, err)
	require.Contains(t, err.Error(), "spec not found")

	// 父表 spec 未声明 reassignable → 拒绝
	writeTestSpec(t, specsDir, "seller_user", "dataScope:\n  mode: auto\n")
	err = validateInheritParent(specsDir, "seller_user")
	require.Error(t, err)
	require.Contains(t, err.Error(), "reassignable=true")

	// 父表 spec 声明 reassignable，但仓库中无 seller_user repo（业务表未生成）
	// → 锚点落点检查拒绝
	writeTestSpec(t, specsDir, "seller_user", "dataScope:\n  mode: auto\n  reassignable: true\n")
	err = validateInheritParent(specsDir, "seller_user")
	require.Error(t, err)
	require.Contains(t, err.Error(), "repository")

	// repo 文件存在但无锚点块（如未带 CascadeOwners() 的裸 package）→ 拒绝
	repoDir := filepath.Join(repoRootOverride, "internal", "admin", "repository")
	require.NoError(t, os.MkdirAll(repoDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "seller_user.go"), []byte("package repository\n"), 0644))
	err = validateInheritParent(specsDir, "seller_user")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no cascade anchor block")

	// repo 文件含锚点块 → 通过
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "seller_user.go"), []byte(anchorFixture("SellerUser")), 0644))
	err = validateInheritParent(specsDir, "seller_user")
	require.NoError(t, err)

	// 硬契约：owner 列非 admin_id → 拒绝（owner 校验先于 repo 落点检查）
	writeTestSpec(t, specsDir, "seller_user", "dataScope:\n  mode: required\n  ownerColumn: agent_id\n  assignOnCreate: true\n  reassignable: true\n")
	err = validateInheritParent(specsDir, "seller_user")
	require.Error(t, err)
	require.Contains(t, err.Error(), `owner column "agent_id"`)
	require.Contains(t, err.Error(), "requires admin_id")

	// 硬契约：主键非 id → 拒绝（pk 校验先于 repo 落点检查）
	nonIDSpec := "name: seller_user\ncomment: test\ndataScope:\n  mode: auto\n  reassignable: true\n" +
		"fields:\n" +
		"  - name: user_id\n    type: bigint\n    unsigned: true\n    primaryKey: true\n    autoIncrement: true\n    null: false\n    comment: ID\n" +
		"  - name: admin_id\n    type: int\n    unsigned: true\n    null: false\n    comment: 管理员ID\n"
	if err := os.WriteFile(filepath.Join(specsDir, "seller_user.yaml"), []byte(nonIDSpec), 0644); err != nil {
		t.Fatal(err)
	}
	err = validateInheritParent(specsDir, "seller_user")
	require.Error(t, err)
	require.Contains(t, err.Error(), "primary key must be exactly id")
}

// TestValidateInheritParentRealUser 使用仓库真实 crud_specs/ 与手写 user repo
//（registerOnly 登记 + 锚点块）验证通过路径——重装后 crud_log 为空也能通过。
func TestValidateInheritParentRealUser(t *testing.T) {
	require.NoError(t, validateInheritParent(DefaultSpecsDir(), "user"))
}

// TestFindInheritReferrers 验证 inbound inheritFrom 引用扫描（specs 驱动）：
// 无引用→空；有子表声明指向主表→列出（含多子表排序）；主实体自身 spec 不计入。
func TestFindInheritReferrers(t *testing.T) {
	specsDir := t.TempDir()

	// 无任何引用 → 空
	writeTestSpec(t, specsDir, "seller_user", "dataScope:\n  mode: auto\n  reassignable: true\n")
	referrers, err := findInheritReferrers(specsDir, "seller_user")
	require.NoError(t, err)
	require.Empty(t, referrers)

	// 主实体自身 spec 不计入引用；一个子表声明指向它
	writeTestSpec(t, specsDir, "seller_user_child_a", "dataScope:\n  mode: auto\n  inheritFrom:\n    table: seller_user\n    byColumn: user_id\n")
	referrers, err = findInheritReferrers(specsDir, "seller_user")
	require.NoError(t, err)
	require.Equal(t, []string{"seller_user_child_a"}, referrers)

	// 多子表 + 确定性排序；指向其它父表的子表不计数
	writeTestSpec(t, specsDir, "seller_user_child_b", "dataScope:\n  mode: auto\n  inheritFrom:\n    table: seller_user\n    byColumn: user_id\n")
	writeTestSpec(t, specsDir, "order_user_child", "dataScope:\n  mode: auto\n  inheritFrom:\n    table: order_user\n    byColumn: user_id\n")
	referrers, err = findInheritReferrers(specsDir, "seller_user")
	require.NoError(t, err)
	require.Equal(t, []string{"seller_user_child_a", "seller_user_child_b"}, referrers)

	// 非法 spec（缺失字段）→ 报错（fail-closed）
	if err := os.WriteFile(filepath.Join(specsDir, "broken.yaml"), []byte("name: broken\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err = findInheritReferrers(specsDir, "seller_user")
	require.Error(t, err)
	require.Contains(t, err.Error(), "broken.yaml")
}
