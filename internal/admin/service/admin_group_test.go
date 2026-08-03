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
	operator := createGroupFixture(t, db, "operator", "1,2", "1")
	attachGroupFixture(t, db, 5, operator.ID)

	// Duplicate ids must collapse before serialization: "1,1,3" would
	// otherwise let len(groupRules) outgrow the real rule set and distort
	// the "all rights + extras" comparison in GetAllAuthGroups.
	rules, err := svc.HandleRules(context.Background(), []int32{1, 1, 3, 3}, 5)
	require.NoError(t, err)
	require.Equal(t, "1,3", rules)
}
