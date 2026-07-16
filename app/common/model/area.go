package model

import (
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BaArea 省份地区表
type Area struct {
	ID        int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"` // ID
	Pid       int32  `gorm:"column:pid;comment:父id" json:"pid"`                            // 父id
	Shortname string `gorm:"column:shortname;comment:简称" json:"shortname"`                 // 简称
	Name      string `gorm:"column:name;comment:名称" json:"name"`                           // 名称
	Mergename string `gorm:"column:mergename;comment:全称" json:"mergename"`                 // 全称
	Level     int32  `gorm:"column:level;comment:层级:1=省,2=市,3=区/县" json:"level"`           // 层级:1=省,2=市,3=区/县
	Pinyin    string `gorm:"column:pinyin;comment:拼音" json:"pinyin"`                       // 拼音
	Code      string `gorm:"column:code;comment:长途区号" json:"code"`                         // 长途区号
	Zip       string `gorm:"column:zip;comment:邮编" json:"zip"`                             // 邮编
	First     string `gorm:"column:first;comment:首字母" json:"first"`                        // 首字母
	Lng       string `gorm:"column:lng;comment:经度" json:"lng"`                             // 经度
	Lat       string `gorm:"column:lat;comment:纬度" json:"lat"`                             // 纬度
}

type AreaModel struct {
	BaseModel
}

func NewAreaModel(sqlDB *gorm.DB, config *conf.Configuration) *AreaModel {
	return &AreaModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "area",
			Key:              "id",
			QuickSearchField: "name",
			sqlDB:            sqlDB,
		},
	}
}

func (s *AreaModel) List(ctx *gin.Context) (any, error) {
	whereS := "pid=? and level=?"
	pid := "0"
	level := "1"
	province := ctx.Request.FormValue("province")
	city := ctx.Request.FormValue("city")
	if province != "" {
		pid = province
		level = "2"
		if city != "" {
			pid = city
			level = "3"
		}
	}

	list := []struct {
		Value int32  `json:"value"`
		Label string `json:"label"`
	}{}
	err := s.sqlDB.Model(&Area{}).Select("id as value,name as label").Where(whereS, pid, level).Scan(&list).Error
	return list, err
}
