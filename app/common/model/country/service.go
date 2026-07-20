package country

import (
	"context"

	"go-build-admin/conf"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Language struct {
	ID     int64  `gorm:"column:id;primaryKey;autoIncrement"`
	Lan    string `gorm:"column:lan"`
	Name   string `gorm:"column:name"`
	Remark string `gorm:"column:remark"`
	Status int8   `gorm:"column:status"`
	Weigh  int32  `gorm:"column:weigh"`
}

type LanguageContent struct {
	ID    int64  `gorm:"column:id;primaryKey;autoIncrement"`
	Lan   string `gorm:"column:lan"`
	Group string `gorm:"column:group"`
	Key   string `gorm:"column:key"`
	Type  int8   `gorm:"column:type"`
	Value string `gorm:"column:value"`
}

const (
	ContentTypeText     int8 = 0
	ContentTypeRichText int8 = 1
	ContentTypeImg      int8 = 2
)

type Currency struct {
	ID     int64   `gorm:"column:id;primaryKey;autoIncrement"`
	Code   string  `gorm:"column:code"`
	Name   string  `gorm:"column:name"`
	Symbol string  `gorm:"column:symbol"`
	Rate   float64 `gorm:"column:rate"`
	Status int8    `gorm:"column:status"`
	Weigh  int32   `gorm:"column:weigh"`
}

type Service struct {
	db     *gorm.DB
	prefix string
}

func NewService(db *gorm.DB, config *conf.Configuration) *Service {
	prefix := ""
	if config != nil {
		prefix = config.Database.Prefix
	}
	return &Service{db: db, prefix: prefix}
}

func (s *Service) Get(ctx context.Context, lan, group, key string) (string, error) {
	var content LanguageContent
	if err := s.findContent(ctx, lan, group, key, &content); err == nil {
		return content.Value, nil
	} else if err != gorm.ErrRecordNotFound {
		return "", err
	}
	defaultLan, err := s.DefaultLan(ctx)
	if err != nil {
		return "", err
	}
	if defaultLan == lan {
		return "", gorm.ErrRecordNotFound
	}
	if err := s.findContent(ctx, defaultLan, group, key, &content); err == nil {
		return content.Value, nil
	} else if err != gorm.ErrRecordNotFound {
		return "", err
	}
	return "", gorm.ErrRecordNotFound
}

func (s *Service) findContent(ctx context.Context, lan, group, key string, content *LanguageContent) error {
	return s.db.WithContext(ctx).Table(s.table("country_language_content")).Where("lan = ? AND `group` = ? AND `key` = ?", lan, group, key).Take(content).Error
}

func (s *Service) BatchUpsert(ctx context.Context, rows []LanguageContent) error {
	return s.db.WithContext(ctx).Table(s.table("country_language_content")).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "lan"}, {Name: "group"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "type"}),
	}).Create(&rows).Error
}

func (s *Service) DefaultLan(ctx context.Context) (string, error) {
	languages, err := s.EnabledLanguages(ctx)
	if err != nil {
		return "", err
	}
	if len(languages) == 0 {
		return "en", nil
	}
	return languages[0].Lan, nil
}

func (s *Service) EnabledLanguages(ctx context.Context) ([]Language, error) {
	var values []Language
	if err := s.db.WithContext(ctx).Table(s.table("country_language")).Where("status = ?", 1).Order("weigh DESC, id ASC").Find(&values).Error; err != nil {
		return nil, err
	}
	return values, nil
}

func (s *Service) EnabledCurrencies(ctx context.Context) ([]Currency, error) {
	var values []Currency
	if err := s.db.WithContext(ctx).Table(s.table("country_currency")).Where("status = ?", 1).Order("weigh DESC, id ASC").Find(&values).Error; err != nil {
		return nil, err
	}
	return values, nil
}

func (s *Service) table(name string) string { return s.prefix + name }
