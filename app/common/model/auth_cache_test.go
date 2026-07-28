package model

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go-build-admin/app/pkg/token"
	"go-build-admin/conf"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type authCacheUserGroup struct {
	ID     int32  `gorm:"column:id;primaryKey"`
	Rules  string `gorm:"column:rules"`
	Status string `gorm:"column:status"`
}

func (authCacheUserGroup) TableName() string { return "pfx_user_group" }

type authCacheUserRule struct {
	ID           int32  `gorm:"column:id;primaryKey"`
	Pid          int32  `gorm:"column:pid"`
	Type         string `gorm:"column:type"`
	Title        string `gorm:"column:title"`
	Name         string `gorm:"column:name"`
	Path         string `gorm:"column:path"`
	Icon         string `gorm:"column:icon"`
	MenuType     string `gorm:"column:menu_type"`
	URL          string `gorm:"column:url"`
	Component    string `gorm:"column:component"`
	NoLoginValid string `gorm:"column:no_login_valid"`
	Extend       string `gorm:"column:extend"`
	Weigh        int32  `gorm:"column:weigh"`
	Status       string `gorm:"column:status"`
}

func (authCacheUserRule) TableName() string { return "pfx_user_rule" }

func TestAuthRuleCacheUsesPrefixAndInvalidation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:common-auth-cache-test?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "pfx_"},
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &authCacheUserGroup{}, &authCacheUserRule{}))
	require.NoError(t, db.Create(&User{ID: 1, GroupID: 1}).Error)
	require.NoError(t, db.Create(&authCacheUserGroup{ID: 1, Rules: "1", Status: "1"}).Error)
	require.NoError(t, db.Create(&authCacheUserRule{ID: 1, Pid: 0, Type: "menu", Title: "Initial", Name: "user/initial", Status: "1", Weigh: 1}).Error)

	config := &conf.Configuration{}
	config.Database.Prefix = "pfx_"
	m := NewAuthModel(db, &token.TokenHelper{Driver: authTestTokenDriver{}}, config)

	_, err = m.GetRuleList(nil, 1)
	require.NoError(t, err)
	require.True(t, m.Check("user/initial", 1, "or"))
	require.NoError(t, db.Table("pfx_user_rule").Where("id=?", 1).Update("name", "user/updated").Error)
	require.True(t, m.Check("user/initial", 1, "or"))

	m.InvalidateAll()
	require.False(t, m.Check("user/initial", 1, "or"))
	_, err = m.GetRuleList(nil, 1)
	require.NoError(t, err)
	require.True(t, m.Check("user/updated", 1, "or"))

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 25; j++ {
				_ = m.Check("user/updated", 1, "or")
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
