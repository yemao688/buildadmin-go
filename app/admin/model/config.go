package model

import (
	"encoding/json"
	"fmt"
	"go-build-admin/conf"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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

var jsonDecodeType = []string{"checkbox", "array", "selects"}
var needContent = []string{"radio", "checkbox", "select", "selects"}

func (s *Config) SetValueAttr(value any, t string) string {
	if slices.Contains(jsonDecodeType, t) {
		if v, err := json.Marshal(value); err == nil {
			return string(v)
		}
	} else if t == "switch" {
		if v, ok := value.(bool); ok && v {
			return "1"
		} else {
			return "0"
		}
	} else if t == "time" {
		return value.(string)
	} else if t == "city" || t == "remoteSelects" {
		cityIds := []string{}
		for _, v := range value.([]interface{}) {
			cityIds = append(cityIds, fmt.Sprintf("%v", v))
		}
		return strings.Join(cityIds, ",")
	}
	return fmt.Sprintf("%v", value)
}

func (s *Config) GetValueAttr() any {
	if slices.Contains(jsonDecodeType, s.Type) {
		resultArr := []any{}
		if len(s.Value) > 0 {
			if s.Type == "checkbox" || s.Type == "selects" {
				if err := json.Unmarshal([]byte(s.Value), &resultArr); err == nil {
					return resultArr
				}
			} else {
				result := []map[string]any{}
				if err := json.Unmarshal([]byte(s.Value), &result); err == nil {
					return result
				}
			}
		}
		return resultArr
	} else if s.Type == "switch" {
		if s.Value == "0" {
			return false
		} else {
			return true
		}
	} else if s.Type == "editor" {
		return s.Value
	} else if s.Type == "city" || s.Type == "remoteSelects" {
		if s.Value == "" {
			return []any{}
		}
		return strings.Split(s.Value, ",")
	}
	return s.Value
}

func (s *Config) GetContentAttr() any {
	if slices.Contains(needContent, s.Type) {
		content := map[string]any{}
		if err := json.Unmarshal([]byte(s.Content), &content); err == nil {
			return content
		}
	}
	return map[string]any{}
}

func (s *Config) GetExtendAttr() any {
	extend := map[string]any{}
	if s.Extend != "" {
		err := json.Unmarshal([]byte(s.Extend), &extend)
		if err == nil {
			delete(extend, "baInputExtend")
		}
	}
	return extend
}

func (s *Config) GetInputExtendAttr() any {
	extend := map[string]any{}
	if s.Extend != "" {
		err := json.Unmarshal([]byte(s.Extend), &extend)
		if err == nil {
			if _, ok := extend["baInputExtend"]; ok {
				return extend["baInputExtend"]
			}
		}
	}
	return extend
}

type ConfigModel struct {
	BaseModel
}

func NewConfigModel(sqlDB *gorm.DB, config *conf.Configuration) *ConfigModel {
	return &ConfigModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "config",
			Key:              "id",
			QuickSearchField: "name",
			DataLimit:        "",
			sqlDB:            sqlDB,
		},
	}
}

func (s *ConfigModel) List(ctx *gin.Context) (list []Config, err error) {
	err = s.sqlDB.Model(&Config{}).Order("`weigh` desc").Find(&list).Error
	return
}

func (s *ConfigModel) Add(ctx *gin.Context, data Config) error {
	err := s.sqlDB.Create(&data).Error
	return err
}

func (s *ConfigModel) Edit(ctx *gin.Context, data Config) error {
	err := s.sqlDB.Save(&data).Error
	return err
}

func (s *ConfigModel) Del(ctx *gin.Context, ids interface{}) error {
	err := s.sqlDB.Model(&Config{}).Where("`id` in ? ", ids).Delete(nil).Error
	return err
}

func (s *ConfigModel) GetOneByName(ctx *gin.Context, name string) (Config, error) {
	var config Config
	err := s.sqlDB.Where("`name`= ? ", name).Take(&config).Error
	return config, err
}

func (s *ConfigModel) GetValueByName(ctx *gin.Context, name string) (string, error) {
	var config Config
	err := s.sqlDB.Where("`name`= ? ", name).Take(&config).Error
	return config.Value, err
}

// 获取键值对模式
func (s *ConfigModel) GetKVByGroup(ctx *gin.Context, group string) (map[string]string, error) {
	var configList []*Config
	err := s.sqlDB.Where("`group`=?", group).Find(&configList).Error
	if err != nil {
		return nil, err
	}

	data := map[string]string{}
	for _, v := range configList {
		data[v.Name] = v.Value
	}
	return data, nil
}
