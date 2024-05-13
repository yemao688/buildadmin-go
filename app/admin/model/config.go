package model

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const TableNameConfig = "ba_config"

type Config struct {
	ID       int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`    // ID
	Name     string `gorm:"column:name;not null;comment:变量名" json:"name"`                    // 变量名
	Group    string `gorm:"column:group;not null;comment:分组" json:"group"`                   // 分组
	Title    string `gorm:"column:title;not null;comment:变量标题" json:"title"`                 // 变量标题
	Tip      string `gorm:"column:tip;not null;comment:变量描述" json:"tip"`                     // 变量描述
	Type     string `gorm:"column:type;not null;comment:变量输入组件类型" json:"type"`               // 变量输入组件类型
	Value    string `gorm:"column:value;comment:变量值" json:"value"`                           // 变量值
	Content  string `gorm:"column:content;comment:字典数据" json:"content"`                      // 字典数据
	Rule     string `gorm:"column:rule;not null;comment:验证规则" json:"rule"`                   // 验证规则
	Extend   string `gorm:"column:extend;not null;comment:扩展属性" json:"extend"`               // 扩展属性
	AllowDel int32  `gorm:"column:allow_del;not null;comment:允许删除:0=否,1=是" json:"allow_del"` // 允许删除:0=否,1=是
	Weigh    int32  `gorm:"column:weigh;not null;comment:权重" json:"weigh"`                   // 权重
}

type ConfigModel struct {
	BaseModel
	sqlDB *gorm.DB
}

func NewConfigModel(sqlDB *gorm.DB) *ConfigModel {
	return &ConfigModel{sqlDB: sqlDB}
}

func (s *ConfigModel) List(ctx *gin.Context) (list []Config, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, err
	}
	err = s.sqlDB.Table(TableNameConfig).Where(whereS, whereP...).Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *ConfigModel) GetOneByName(ctx *gin.Context, name string) (Config, error) {
	var config Config
	err := s.sqlDB.Where("name = ? ", name).Find(&config).Error
	return config, err
}

func (s *ConfigModel) GetValueByName(ctx *gin.Context, name string) (string, error) {
	var config Config
	err := s.sqlDB.Where("name = ? ", name).Find(&config).Error
	return config.Value, err
}

func (s *ConfigModel) GetByGroup(ctx *gin.Context, group string) (map[string]string, error) {
	var configList []Config
	err := s.sqlDB.Where("group=?", group).Find(&configList).Error
	if err != nil {
		return nil, err
	}
	data := map[string]string{}
	for _, v := range configList {
		data[v.Name] = v.Value
	}
	return data, nil
}
