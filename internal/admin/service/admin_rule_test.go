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

func newAdminRuleServiceFixture(t *testing.T) (*AdminRuleService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:admin-rule-service-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	require.NoError(t, err)
	require.NoError(t, testutil.CreateSQLiteAdminRuleTables(db, "admin_rule", "admin_group", "admin_group_access"))
	config := &conf.Configuration{}
	svc := NewAdminRuleService(
		adminauth.NewAdminRuleRepository(db, config),
		adminauth.NewAuthRepository(db, nil, config),
	)
	return svc, db
}

// TestAdminRuleServiceMenusFollowsOperatorScope 验证 Menus 按操作者权限范围
// 过滤：超管（规则集合含 "*"）看到全部，普通管理员只看到自己规则集合内的项。
func TestAdminRuleServiceMenusFollowsOperatorScope(t *testing.T) {
	svc, db := newAdminRuleServiceFixture(t)
	for _, id := range []int32{1, 2, 3} {
		rule := model.AdminRule{ID: id, Type: "button", Title: "rule-" + strconv.Itoa(int(id)), Name: "rule-" + strconv.Itoa(int(id)), Status: "1"}
		require.NoError(t, db.Create(&rule).Error)
	}
	operator := model.AdminGroup{Name: "operator", Rules: "1,2", Status: "1"}
	require.NoError(t, db.Create(&operator).Error)
	require.NoError(t, db.Create(&model.AdminGroupAccess{UID: 5, GroupID: operator.ID}).Error)
	super := model.AdminGroup{Name: "super", Rules: "*", Status: "1"}
	require.NoError(t, db.Create(&super).Error)
	require.NoError(t, db.Create(&model.AdminGroupAccess{UID: 6, GroupID: super.ID}).Error)

	list, err := svc.Menus(context.Background(), 5, "", nil, "")
	require.NoError(t, err)
	ids := make([]int32, 0, len(list))
	for _, rule := range list {
		ids = append(ids, rule.ID)
	}
	require.ElementsMatch(t, []int32{1, 2}, ids)

	list, err = svc.Menus(context.Background(), 6, "", nil, "")
	require.NoError(t, err)
	require.Len(t, list, 3)
}
