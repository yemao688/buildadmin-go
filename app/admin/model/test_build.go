package model

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const TableNameTestBuild = "ba_test_build"

type TestBuild struct {
	ID        int64   `gorm:"column:id;primaryKey;comment:ID" json:"id"`                                                // ID
	Image     string  `gorm:"column:image;not null;comment:图片" json:"image"`                                            // 图片
	File      string  `gorm:"column:file;not null;comment:文件" json:"file"`                                              // 文件
	Radio     string  `gorm:"column:radio;not null;default:opt0;comment:单选框:opt0=选项一,opt1=选项二" json:"radio"`            // 单选框:opt0=选项一,opt1=选项二
	Checkbox  string  `gorm:"column:checkbox;not null;default:opt0,opt1;comment:复选框:opt0=选项一,opt1=选项二" json:"checkbox"` // 复选框:opt0=选项一,opt1=选项二
	Select    string  `gorm:"column:select;not null;default:opt0;comment:下拉框:opt0=选项一,opt1=选项二" json:"select"`          // 下拉框:opt0=选项一,opt1=选项二
	Switch    bool    `gorm:"column:switch;not null;default:1;comment:开关:0=关,1=开" json:"switch"`                        // 开关:0=关,1=开
	Editor    string  `gorm:"column:editor;comment:富文本" json:"editor"`                                                  // 富文本
	Textarea  string  `gorm:"column:textarea;not null;comment:多行文本框" json:"textarea"`                                   // 多行文本框
	Float     float64 `gorm:"column:float;not null;default:0.00;comment:浮点数" json:"float"`                              // 浮点数
	Password  string  `gorm:"column:password;not null;comment:密码" json:"password"`                                      // 密码
	Array     string  `gorm:"column:array;not null;comment:数组" json:"array"`                                            // 数组
	Icon      string  `gorm:"column:icon;not null;comment:图标选择" json:"icon"`                                            // 图标选择
	BannerIds string  `gorm:"column:banner_ids;not null;comment:远程下拉" json:"banner_ids"`                                // 远程下拉
}

func (TestBuild) TableName() string {
	return TableNameTestBuild
}

func (TestBuild) Key() string {
	return "id"
}

func (TestBuild) QuickSearchField() string {
	return "id"
}

type TestBuildModel struct {
	sqlDB *gorm.DB
}

func NewTestBuildModel(sqlDB *gorm.DB) *TestBuildModel {
	return &TestBuildModel{sqlDB: sqlDB}
}

func (s *TestBuildModel) List(ctx *gin.Context) (list []TestBuild, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, TestBuild{}, nil)
	if err != nil {
		return nil, err
	}
	err = s.sqlDB.Table(TableNameTestBuild).Where(whereS, whereP...).Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *TestBuildModel) Add(ctx *gin.Context, data TestBuild) error {
	result := s.sqlDB.Create(&data)
	return result.Error
}

func (s *TestBuildModel) Edit(ctx *gin.Context, data map[string]interface{}) error {
	result := s.sqlDB.Model(&TestBuild{}).Updates(data)
	return result.Error
}

func (s *TestBuildModel) Del(ctx *gin.Context, id int64) error {
	result := s.sqlDB.Delete(&TestBuild{}, id)
	return result.Error
}
