package repository

import (
	"context"
	"strconv"
	"testing"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func newAdminRuleRepositoryFixture(t *testing.T) *AdminRuleRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:admin-rule-repo-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	require.NoError(t, err)
	require.NoError(t, testutil.CreateSQLiteAdminRuleTables(db, "admin_rule", "admin_group", "admin_group_access"))
	return NewAdminRuleRepository(db, &conf.Configuration{})
}

func seedAdminRule(t *testing.T, m *AdminRuleRepository, id int32, ruleType, status string, weigh int32) {
	t.Helper()
	rule := model.AdminRule{
		ID:     id,
		Type:   ruleType,
		Title:  "rule-" + strconv.Itoa(int(id)),
		Name:   "rule-" + strconv.Itoa(int(id)),
		Status: status,
		Weigh:  weigh,
	}
	require.NoError(t, m.DB().Create(&rule).Error)
}

func TestAdminRuleListForMenuFiltersAndOrders(t *testing.T) {
	m := newAdminRuleRepositoryFixture(t)
	seedAdminRule(t, m, 1, "menu_dir", "1", 10)
	seedAdminRule(t, m, 2, "menu", "0", 20)
	seedAdminRule(t, m, 3, "button", "1", 30)

	t.Run("super admin sees every rule ordered by weigh", func(t *testing.T) {
		list, err := m.ListForMenu(context.Background(), true, nil, "", nil, "")
		require.NoError(t, err)
		require.Len(t, list, 3)
		require.Equal(t, int32(3), list[0].ID)
		require.Equal(t, int32(1), list[2].ID)
	})

	t.Run("non-super is limited to the given rule ids", func(t *testing.T) {
		list, err := m.ListForMenu(context.Background(), false, []int32{1, 3}, "", nil, "")
		require.NoError(t, err)
		require.Len(t, list, 2)
	})

	t.Run("type and status narrow the result", func(t *testing.T) {
		list, err := m.ListForMenu(context.Background(), true, nil, "", []string{"menu_dir", "menu"}, "1")
		require.NoError(t, err)
		require.Len(t, list, 1)
		require.Equal(t, int32(1), list[0].ID)
	})

	t.Run("keyword terms are AND-ed", func(t *testing.T) {
		list, err := m.ListForMenu(context.Background(), true, nil, "rule-2", nil, "")
		require.NoError(t, err)
		require.Len(t, list, 1)
		require.Equal(t, int32(2), list[0].ID)

		list, err = m.ListForMenu(context.Background(), true, nil, "rule-2 missing", nil, "")
		require.NoError(t, err)
		require.Empty(t, list)
	})
}
