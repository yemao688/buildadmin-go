package model

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go-build-admin/conf"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func newAdminAuthCacheModel(t *testing.T) (*AuthModel, *gorm.DB, AdminRule) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:admin-auth-cache-"+strconv.FormatInt(time.Now().UnixNano(), 10)+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	require.NoError(t, db.AutoMigrate(&AdminRule{}, &AdminGroup{}, &AdminGroupAccess{}))

	rule := AdminRule{Pid: 0, Type: "menu", Title: "Initial", Name: "auth/initial", Status: "1", Weigh: 1}
	require.NoError(t, db.Create(&rule).Error)
	require.NoError(t, db.Create(&AdminGroup{ID: 1, Name: "operators", Rules: strconv.Itoa(int(rule.ID)), Status: "1"}).Error)
	require.NoError(t, db.Create(&AdminGroupAccess{UID: 1, GroupID: 1}).Error)

	config := &conf.Configuration{}
	return NewAuthModel(db, nil, config), db, rule
}

func TestAdminAuthCacheInvalidationReloadsRules(t *testing.T) {
	m, db, rule := newAdminAuthCacheModel(t)

	_, err := m.GetRuleList(nil, 1)
	require.NoError(t, err)
	require.True(t, m.Check("auth/initial", 1, "or"))

	require.NoError(t, db.Model(&AdminRule{}).Where("id=?", rule.ID).Update("name", "auth/updated").Error)
	require.True(t, m.Check("auth/initial", 1, "or"))

	m.InvalidateUser(1)
	require.False(t, m.Check("auth/initial", 1, "or"))
	_, err = m.GetRuleList(nil, 1)
	require.NoError(t, err)
	require.False(t, m.Check("auth/initial", 1, "or"))
	require.True(t, m.Check("auth/updated", 1, "or"))
}

func TestAdminAuthCacheCopiesGroupsAndInvalidatesGroupMembership(t *testing.T) {
	m, db, _ := newAdminAuthCacheModel(t)

	groups, err := m.GetGroups(1)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	groups[0].Rules = "changed"
	groups, err = m.GetGroups(1)
	require.NoError(t, err)
	require.NotEqual(t, "changed", groups[0].Rules)

	ids, err := m.GetRuleIds(1)
	require.NoError(t, err)
	require.Equal(t, []string{"1"}, ids)
	require.NoError(t, db.Model(&AdminGroup{}).Where("id=?", 1).Update("rules", "99").Error)
	ids, err = m.GetRuleIds(1)
	require.NoError(t, err)
	require.Equal(t, []string{"1"}, ids)
	m.InvalidateUser(1)
	ids, err = m.GetRuleIds(1)
	require.NoError(t, err)
	require.Equal(t, []string{"99"}, ids)
}

func TestAdminAuthCacheConcurrentAccess(t *testing.T) {
	m, _, _ := newAdminAuthCacheModel(t)
	_, err := m.GetRuleList(nil, 1)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 25; j++ {
				_ = m.Check("auth/initial", 1, "or")
				_, _ = m.GetMenus(nil, 1)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 25; i++ {
			m.InvalidateAll()
			_, _ = m.GetRuleList(nil, 1)
		}
	}()
	wg.Wait()
}
