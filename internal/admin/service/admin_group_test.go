package service

import (
	"context"
	"strconv"
	"testing"

	adminauth "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func newAdminGroupServiceFixture(t *testing.T) (*AdminGroupService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:admin-group-service-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	require.NoError(t, err)
	require.NoError(t, testutil.CreateSQLiteAdminRuleTables(db, "admin_rule", "admin_group", "admin_group_access"))
	config := &conf.Configuration{}
	svc := NewAdminGroupService(
		adminauth.NewAdminGroupRepository(db, config),
		adminauth.NewAdminRuleRepository(db, config),
		adminauth.NewAuthRepository(db, nil, config),
	)
	return svc, db
}

func createGroupFixture(t *testing.T, db *gorm.DB, name, rules, status string) model.AdminGroup {
	t.Helper()
	group := model.AdminGroup{Name: name, Rules: rules, Status: status}
	require.NoError(t, db.Create(&group).Error)
	return group
}

func createRuleFixture(t *testing.T, db *gorm.DB, id int32) {
	t.Helper()
	rule := model.AdminRule{ID: id, Type: "button", Title: "rule-" + strconv.Itoa(int(id)), Name: "rule-" + strconv.Itoa(int(id)), Status: "1"}
	require.NoError(t, db.Create(&rule).Error)
}

func attachGroupFixture(t *testing.T, db *gorm.DB, uid, groupID int32) {
	t.Helper()
	require.NoError(t, db.Create(&model.AdminGroupAccess{UID: uid, GroupID: groupID}).Error)
}

// switchAuthFixture seeds rules 1..3, an operator (uid 5) holding rules 1+2,
// and a target group carrying exactly rule 1 (a strict subset, so CheckAuth
// authorizes uid 5).
func switchAuthFixture(t *testing.T, db *gorm.DB, targetRules string) (target model.AdminGroup) {
	t.Helper()
	createRuleFixture(t, db, 1)
	createRuleFixture(t, db, 2)
	createRuleFixture(t, db, 3)
	operator := createGroupFixture(t, db, "operator", "1,2", "1")
	attachGroupFixture(t, db, 5, operator.ID)
	return createGroupFixture(t, db, "target", targetRules, "1")
}

func TestAdminGroupServiceSwitchStatusAllowsAuthorizedOperator(t *testing.T) {
	svc, db := newAdminGroupServiceFixture(t)
	target := switchAuthFixture(t, db, "1")

	require.NoError(t, svc.SwitchStatus(context.Background(), target.ID, "0", 5, false))

	var reloaded model.AdminGroup
	require.NoError(t, db.First(&reloaded, target.ID).Error)
	require.Equal(t, "0", reloaded.Status)
}

func TestAdminGroupServiceSwitchStatusRejectsUnauthorizedOperator(t *testing.T) {
	svc, db := newAdminGroupServiceFixture(t)
	// The target group carries rule 3, outside the operator's rule set, so
	// the operator is not authorized to operate it.
	target := switchAuthFixture(t, db, "3")

	err := svc.SwitchStatus(context.Background(), target.ID, "0", 5, false)
	require.EqualError(t, err, "You need to have all the permissions of the group and have additional permissions before you can operate the group~")

	var reloaded model.AdminGroup
	require.NoError(t, db.First(&reloaded, target.ID).Error)
	require.Equal(t, "1", reloaded.Status)
}

func TestAdminGroupServiceSwitchStatusRejectsSelfGroup(t *testing.T) {
	svc, db := newAdminGroupServiceFixture(t)
	target := switchAuthFixture(t, db, "1")
	// The operator is a member of the very group it tries to switch.
	attachGroupFixture(t, db, 5, target.ID)

	err := svc.SwitchStatus(context.Background(), target.ID, "0", 5, false)
	require.EqualError(t, err, "You cannot modify your own management group!")

	var reloaded model.AdminGroup
	require.NoError(t, db.First(&reloaded, target.ID).Error)
	require.Equal(t, "1", reloaded.Status)
}

func TestAdminGroupServiceSwitchStatusAllowsSuperAdmin(t *testing.T) {
	svc, db := newAdminGroupServiceFixture(t)
	target := switchAuthFixture(t, db, "3")

	require.NoError(t, svc.SwitchStatus(context.Background(), target.ID, "0", 5, true))

	var reloaded model.AdminGroup
	require.NoError(t, db.First(&reloaded, target.ID).Error)
	require.Equal(t, "0", reloaded.Status)
}

func TestAdminGroupServiceHandleRulesDeduplicates(t *testing.T) {
	svc, db := newAdminGroupServiceFixture(t)
	createRuleFixture(t, db, 1)
	createRuleFixture(t, db, 2)
	createRuleFixture(t, db, 3)
	// operator(5) 拥有规则 1,2,3，提交 1,3 不超出其可分配范围。
	operator := createGroupFixture(t, db, "operator", "1,2,3", "1")
	attachGroupFixture(t, db, 5, operator.ID)

	// Duplicate ids must collapse before serialization: "1,1,3" would
	// otherwise let len(groupRules) outgrow the real rule set and distort
	// the "all rights + extras" comparison in GetAllAuthGroups.
	rules, err := svc.HandleRules(context.Background(), []int32{1, 1, 3, 3}, 5, false)
	require.NoError(t, err)
	require.Equal(t, "1,3", rules)
}

// TestAdminGroupServiceHandleRulesRejectsAllRightsForNonSuper 对齐 PHP 上游
// Group::handleRules：非超管提交"拥有自己全部权限"的分组被拒绝（防止复制
// 自己的权限集给下级）。
func TestAdminGroupServiceHandleRulesRejectsAllRightsForNonSuper(t *testing.T) {
	svc, db := newAdminGroupServiceFixture(t)
	createRuleFixture(t, db, 1)
	createRuleFixture(t, db, 2)
	// operator(5) 只拥有规则 1,2
	operator := createGroupFixture(t, db, "operator", "1,2", "1")
	attachGroupFixture(t, db, 5, operator.ID)

	// 提交 1,2 = 拥有自己全部权限 → 拒绝
	_, err := svc.HandleRules(context.Background(), []int32{1, 2}, 5, false)
	require.EqualError(t, err, "Role group has all your rights, please contact the upper administrator to add or do not need to add!")
}

// TestAdminGroupServiceHandleRulesRejectsOutOfRangeForNonSuper 对齐 PHP 上游：
// 非超管提交超出自己可分配范围的分组权限被拒绝。
func TestAdminGroupServiceHandleRulesRejectsOutOfRangeForNonSuper(t *testing.T) {
	svc, db := newAdminGroupServiceFixture(t)
	createRuleFixture(t, db, 1)
	createRuleFixture(t, db, 2)
	createRuleFixture(t, db, 3)
	// operator(5) 只拥有规则 1,2
	operator := createGroupFixture(t, db, "operator", "1,2", "1")
	attachGroupFixture(t, db, 5, operator.ID)

	// 提交 1,3：3 超出 operator 可分配范围 → 拒绝
	_, err := svc.HandleRules(context.Background(), []int32{1, 3}, 5, false)
	require.EqualError(t, err, "The group permission node exceeds the range that can be allocated")
}

// TestAdminGroupServiceHandleRulesAllowsSuperAdminStar 对齐 PHP 上游：仅超管
// 提交全部规则时获得超管级分组（rules="*"）；非超管即使提交全部规则也被
// "拥有自己全部权限"规则拒绝。
func TestAdminGroupServiceHandleRulesAllowsSuperAdminStar(t *testing.T) {
	svc, db := newAdminGroupServiceFixture(t)
	createRuleFixture(t, db, 1)
	createRuleFixture(t, db, 2)
	createRuleFixture(t, db, 3)

	// 超管提交全部规则 → "*"
	rules, err := svc.HandleRules(context.Background(), []int32{1, 2, 3}, 5, true)
	require.NoError(t, err)
	require.Equal(t, "*", rules)

	// 非超管提交全部规则 → 拒绝（拥有自己全部权限）
	operator := createGroupFixture(t, db, "operator", "1,2", "1")
	attachGroupFixture(t, db, 5, operator.ID)
	_, err = svc.HandleRules(context.Background(), []int32{1, 2, 3}, 5, false)
	require.EqualError(t, err, "Role group has all your rights, please contact the upper administrator to add or do not need to add!")
}

// TestAdminGroupServiceListForOperatorDecoratesRules 验证列表展示文本由
// service 批量补全：超管标识、首条规则标题（多权限追加"等 N 项"）与无权限
// 文案，且 i18n 通过 translate 回调注入。
func TestAdminGroupServiceListForOperatorDecoratesRules(t *testing.T) {
	svc, db := newAdminGroupServiceFixture(t)
	createRuleFixture(t, db, 1)
	createRuleFixture(t, db, 2)
	createGroupFixture(t, db, "super", "*", "1")
	createGroupFixture(t, db, "none", "", "1")
	createGroupFixture(t, db, "single", "1", "1")
	createGroupFixture(t, db, "multi", "1,2", "1")

	list, err := svc.ListForOperator(context.Background(), GroupListQuery{OperatorID: 5, IsSuperAdmin: true}, func(messageID string) string {
		return map[string]string{
			"Super administrator": "超管",
			"no permission":       "无权限",
		}[messageID]
	})
	require.NoError(t, err)

	rules := map[string]string{}
	for _, group := range list {
		rules[group.Name] = group.Rules
	}
	require.Equal(t, "超管", rules["super"])
	require.Equal(t, "无权限", rules["none"])
	require.Equal(t, "rule-1", rules["single"])
	require.Equal(t, "rule-1等 2 项", rules["multi"])
}

// TestAdminGroupServiceListForOperatorFiltersByAuthorization 验证非超管的可见
// 范围：普通列表合并自己所在分组，absoluteAuth 只保留有资格管理的分组。
func TestAdminGroupServiceListForOperatorFiltersByAuthorization(t *testing.T) {
	svc, db := newAdminGroupServiceFixture(t)
	createRuleFixture(t, db, 1)
	createRuleFixture(t, db, 2)
	createGroupFixture(t, db, "super", "*", "1")
	operator := createGroupFixture(t, db, "operator", "1,2", "1")
	attachGroupFixture(t, db, 5, operator.ID)
	createGroupFixture(t, db, "subset", "1", "1")

	names := func(absoluteAuth bool) []string {
		list, err := svc.ListForOperator(context.Background(), GroupListQuery{OperatorID: 5, AbsoluteAuth: absoluteAuth}, nil)
		require.NoError(t, err)
		result := make([]string, 0, len(list))
		for _, group := range list {
			result = append(result, group.Name)
		}
		return result
	}

	require.ElementsMatch(t, []string{"operator", "subset"}, names(false))
	require.ElementsMatch(t, []string{"subset"}, names(true))
}
