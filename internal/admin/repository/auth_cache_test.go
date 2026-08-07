package repository

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func newAdminAuthCacheModel(t *testing.T) (*AuthRepository, *gorm.DB, model.AdminRule) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:admin-auth-cache-"+strconv.FormatInt(time.Now().UnixNano(), 10)+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The entities carry MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture tables with
	// sqlite-native DDL matching the runtime column shape.
	require.NoError(t, testutil.CreateSQLiteAdminRuleTables(db, "admin_rule", "admin_group", "admin_group_access"))

	rule := model.AdminRule{Pid: 0, Type: "menu", Title: "Initial", Name: "auth/initial", Status: "1", Weigh: 1}
	require.NoError(t, db.Create(&rule).Error)
	require.NoError(t, db.Create(&model.AdminGroup{ID: 1, Name: "operators", Rules: strconv.Itoa(int(rule.ID)), Status: "1"}).Error)
	require.NoError(t, db.Create(&model.AdminGroupAccess{UID: 1, GroupID: 1}).Error)

	config := &conf.Configuration{}
	return NewAuthRepository(db, nil, config), db, rule
}

func TestAdminAuthCacheInvalidationReloadsRules(t *testing.T) {
	m, db, rule := newAdminAuthCacheModel(t)

	_, err := m.GetRuleList(nil, 1)
	require.NoError(t, err)
	require.True(t, m.Check("auth/initial", 1, "or"))

	require.NoError(t, db.Model(&model.AdminRule{}).Where("id=?", rule.ID).Update("name", "auth/updated").Error)
	require.True(t, m.Check("auth/initial", 1, "or"))

	m.InvalidateUser(1)
	require.False(t, m.Check("auth/initial", 1, "or"))
	_, err = m.GetRuleList(nil, 1)
	require.NoError(t, err)
	require.False(t, m.Check("auth/initial", 1, "or"))
	require.True(t, m.Check("auth/updated", 1, "or"))
}

func TestAdminAuthCheckDeniesRuleOutsideGroup(t *testing.T) {
	m, db, _ := newAdminAuthCacheModel(t)

	// The group only grants "auth/initial"; an enabled rule that is not part of
	// the group's rules must not pass Check. Regression: the unassigned
	// tx.Where("id in ?") previously dropped the group filter and loaded every
	// enabled rule.
	require.NoError(t, db.Create(&model.AdminRule{Pid: 0, Type: "button", Title: "Other", Name: "auth/other", Status: "1", Weigh: 1}).Error)

	_, err := m.GetRuleList(nil, 1)
	require.NoError(t, err)
	require.True(t, m.Check("auth/initial", 1, "or"))
	require.False(t, m.Check("auth/other", 1, "or"))
}

func TestAdminAuthCheckNormalizesCamelCaseRuleNames(t *testing.T) {
	m, db, rule := newAdminAuthCacheModel(t)

	camelRule := model.AdminRule{Pid: 0, Type: "button", Title: "LanguageContent", Name: "country/languageContent/index", Status: "1", Weigh: 1}
	require.NoError(t, db.Create(&camelRule).Error)
	require.NoError(t, db.Model(&model.AdminGroup{}).Where("id=?", 1).Update("rules", strconv.Itoa(int(rule.ID))+","+strconv.Itoa(int(camelRule.ID))).Error)

	_, err := m.GetRuleList(nil, 1)
	require.NoError(t, err)
	// The middleware builds rule names from normalized (lowercased) route/action
	// pairs; camelCase rule names stored in admin_rule must still match.
	require.True(t, m.Check("country/languagecontent/index", 1, "or"))
	require.True(t, m.Check("country/languageContent/index", 1, "or"))

	names, err := m.GetAllRuleNames()
	require.NoError(t, err)
	require.Contains(t, names, "country/languagecontent/index")
	require.NotContains(t, names, "country/languageContent/index")
}

func TestAdminAuthCheckSuperAdminWildcard(t *testing.T) {
	m, db, _ := newAdminAuthCacheModel(t)
	require.NoError(t, db.Create(&model.AdminGroup{ID: 2, Name: "super", Rules: "*", Status: "1"}).Error)
	require.NoError(t, db.Create(&model.AdminGroupAccess{UID: 2, GroupID: 2}).Error)

	_, err := m.GetRuleList(nil, 2)
	require.NoError(t, err)
	// A group with Rules="*" grants every permission via the wildcard entry.
	require.True(t, m.Check("anything/at/all", 2, "or"))
	require.True(t, m.Check("", 2, "or"))
	require.True(t, m.Check("auth/initial", 2, "and"))
}

func TestAdminAuthCheckCommaSeparatedNamesAndRelations(t *testing.T) {
	m, db, rule := newAdminAuthCacheModel(t)
	second := model.AdminRule{Pid: 0, Type: "button", Title: "Second", Name: "auth/second", Status: "1", Weigh: 2}
	require.NoError(t, db.Create(&second).Error)
	require.NoError(t, db.Model(&model.AdminGroup{}).Where("id=?", 1).Update("rules", strconv.Itoa(int(rule.ID))+","+strconv.Itoa(int(second.ID))).Error)

	_, err := m.GetRuleList(nil, 1)
	require.NoError(t, err)

	// "or": one granted name is enough.
	require.True(t, m.Check("auth/initial,auth/second", 1, "or"))
	require.True(t, m.Check("auth/initial,auth/missing", 1, "or"))
	require.False(t, m.Check("auth/missing,auth/other", 1, "or"))
	// "and": the inherited relation semantics break only on the first missing
	// name before any match; a match preceding a later missing name still
	// returns true (control flow preserved byte-for-byte from the original).
	require.True(t, m.Check("auth/initial,auth/second", 1, "and"))
	require.True(t, m.Check("auth/initial,auth/missing", 1, "and")) // inherited quirk
	require.False(t, m.Check("auth/missing,auth/initial", 1, "and"))
	require.False(t, m.Check("auth/missing,auth/other", 1, "and"))
	// Names are matched case-insensitively, like the stored normalization.
	require.True(t, m.Check("AUTH/INITIAL,auth/second", 1, "and"))
}

func TestAdminAuthHasRuleNameExactNormalizedMatch(t *testing.T) {
	m, db, _ := newAdminAuthCacheModel(t)
	require.NoError(t, db.Create(&model.AdminRule{Pid: 0, Type: "button", Title: "LanguageContent", Name: "country/languageContent/index", Status: "1", Weigh: 1}).Error)

	ok, err := m.HasRuleName("country/languagecontent/index")
	require.NoError(t, err)
	require.True(t, ok)
	// The lookup is exact against the normalized set: an unlowered input does
	// not match, mirroring the previous GetAllRuleNames + slices.Contains
	// membership check. The authorization middleware only ever looks up the
	// lowercased route/action pair produced by NormalizeRouteAction.
	ok, err = m.HasRuleName("country/languageContent/index")
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = m.HasRuleName("auth/unknown")
	require.NoError(t, err)
	require.False(t, ok)
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
	require.NoError(t, db.Model(&model.AdminGroup{}).Where("id=?", 1).Update("rules", "99").Error)
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
