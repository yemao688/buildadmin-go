package model

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const TableNameTestBuild = "ba_test_build"

type TestBuild struct {
	ID           int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`        // ID
	Title        string `gorm:"column:title;not null;comment:标题" json:"title"`                       // 标题
	KeywordRows  string `gorm:"column:keyword_rows;not null;comment:关键词" json:"keyword_rows"`        // 关键词
	Content      string `gorm:"column:content;comment:内容" json:"content"`                            // 内容
	Views        int32  `gorm:"column:views;not null;comment:浏览量" json:"views"`                      // 浏览量
	Likes        int32  `gorm:"column:likes;not null;comment:有帮助数" json:"likes"`                     // 有帮助数
	Dislikes     int32  `gorm:"column:dislikes;not null;comment:无帮助数" json:"dislikes"`               // 无帮助数
	NoteTextarea string `gorm:"column:note_textarea;not null;comment:备注" json:"note_textarea"`       // 备注
	Status       string `gorm:"column:status;not null;default:1;comment:状态:0=隐藏,1=正常" json:"status"` // 状态:0=隐藏,1=正常
	Weigh        int32  `gorm:"column:weigh;not null;comment:权重" json:"weigh"`                       // 权重
	UpdateTime   int64  `gorm:"autoUpdateTime;column:update_time;comment:更新时间" json:"update_time"`   // 更新时间
	CreateTime   int64  `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"`   // 创建时间
}

type TestBuildModel struct {
	BaseModel
}

func NewTestBuildModel(sqlDB *gorm.DB) *TestBuildModel {
	return &TestBuildModel{
		BaseModel: BaseModel{
			TableName:        TableNameTestBuild,
			Key:              "id",
			QuickSearchField: "id",
			DataLimit:        "",
			sqlDB:            sqlDB,
		},
	}
}

func (s *TestBuildModel) List(ctx *gin.Context) (list []TestBuild, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.sqlDB.Table(s.TableName).Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *TestBuildModel) Add(ctx *gin.Context, data TestBuild) error {
	err := s.sqlDB.Table(s.TableName).Create(&data).Error
	return err
}

func (s *TestBuildModel) Edit(ctx *gin.Context, data TestBuild) error {
	err := s.sqlDB.Table(s.TableName).Omit("").Updates(&data).Error
	return err
}

func (s *TestBuildModel) Del(ctx *gin.Context, ids interface{}) error {
	err := s.sqlDB.Table(s.TableName).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Delete(nil).Error
	return err
}
