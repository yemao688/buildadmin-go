package country

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go-build-admin/conf"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestServiceDictionaryContract(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:country-i18n?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "dw_"},
	})
	require.NoError(t, err)
	require.NoError(t, db.Table("dw_country_language").AutoMigrate(&Language{}))
	require.NoError(t, db.Table("dw_country_language_content").AutoMigrate(&LanguageContent{}))
	require.NoError(t, db.Table("dw_country_currency").AutoMigrate(&Currency{}))
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX `uk_country_language_lan` ON `dw_country_language` (`lan`)").Error)
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX `uk_country_currency_code` ON `dw_country_currency` (`code`)").Error)
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX `uk_country_language_content_lan_group_key` ON `dw_country_language_content` (`lan`, `group`, `key`)").Error)

	s := NewService(db, &conf.Configuration{Database: conf.Database{Prefix: "dw_"}})
	ctx := context.Background()
	require.NoError(t, db.Table("dw_country_language").Create(&[]Language{
		{Lan: "zh", Name: "Chinese", Status: 1, Weigh: 10},
		{Lan: "en", Name: "English", Status: 1, Weigh: 1},
	}).Error)
	require.NoError(t, s.BatchUpsert(ctx, []LanguageContent{
		{Lan: "zh", Group: "site", Key: "title", Type: ContentTypeText, Value: "标题"},
		{Lan: "en", Group: "site", Key: "title", Type: ContentTypeText, Value: "Title"},
	}))

	value, err := s.Get(ctx, "zh", "site", "title")
	require.NoError(t, err)
	require.Equal(t, "标题", value)
	value, err = s.Get(ctx, "fr", "site", "title")
	require.NoError(t, err)
	require.Equal(t, "标题", value)

	require.NoError(t, db.Table("dw_country_language").Where("status = ?", 1).Update("status", 0).Error)
	defaultLan, err := s.DefaultLan(ctx)
	require.NoError(t, err)
	require.Equal(t, "en", defaultLan)
	value, err = s.Get(ctx, "fr", "site", "title")
	require.NoError(t, err)
	require.Equal(t, "Title", value)
	_, err = s.Get(ctx, "fr", "site", "missing")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	require.NoError(t, s.BatchUpsert(ctx, []LanguageContent{{Lan: "en", Group: "site", Key: "title", Type: ContentTypeRichText, Value: "Updated"}}))
	require.NoError(t, s.BatchUpsert(ctx, []LanguageContent{{Lan: "en", Group: "site", Key: "title", Type: ContentTypeImg, Value: "Updated again"}}))
	var count int64
	require.NoError(t, db.Table("dw_country_language_content").Where("lan = ? AND `group` = ? AND `key` = ?", "en", "site", "title").Count(&count).Error)
	require.Equal(t, int64(1), count)

	_, err = s.EnabledLanguages(ctx)
	require.NoError(t, err)
	require.NoError(t, db.Table("dw_country_language").Create(&Language{Lan: "ja", Name: "Japanese", Status: 1, Weigh: 20}).Error)
	languages, err := s.EnabledLanguages(ctx)
	require.NoError(t, err)
	require.Len(t, languages, 1)
	require.Equal(t, "ja", languages[0].Lan)
}
