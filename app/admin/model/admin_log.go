package model

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const TableNameAdminLog = "ba_admin_log"

type AdminLog struct {
	ID         int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`  // ID
	AdminID    int32  `gorm:"column:admin_id;not null;comment:管理员ID" json:"admin_id"`        // 管理员ID
	Username   string `gorm:"column:username;not null;comment:管理员用户名" json:"username"`       // 管理员用户名
	URL        string `gorm:"column:url;not null;comment:操作Url" json:"url"`                  // 操作Url
	Title      string `gorm:"column:title;not null;comment:日志标题" json:"title"`               // 日志标题
	Data       string `gorm:"column:data;comment:请求数据" json:"data"`                          // 请求数据
	IP         string `gorm:"column:ip;not null;comment:IP" json:"ip"`                       // IP
	Useragent  string `gorm:"column:useragent;not null;comment:User-Agent" json:"useragent"` // User-Agent
	CreateTime int64  `gorm:"column:create_time;comment:创建时间" json:"create_time"`            // 创建时间
}

func (AdminLog) TableName() string {
	return TableNameAdminLog
}

func (AdminLog) Key() string {
	return "id"
}

func (AdminLog) QuickSearchField() string {
	return "id"
}

type AdminLogModel struct {
	sqlDB *gorm.DB
}

func NewAdminLogModel(sqlDB *gorm.DB) *AdminLogModel {
	return &AdminLogModel{sqlDB: sqlDB}
}

func (s *AdminLogModel) List(ctx *gin.Context) (list []AdminLog, err error) {
	var adminLog AdminLog
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, adminLog, nil)
	if err != nil {
		return nil, err
	}
	err = s.sqlDB.Table(TableNameAdminLog).Where(whereS, whereP...).Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *AdminLogModel) Add(ctx *gin.Context) (list []AdminLog, err error) {
	var adminLog AdminLog
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, adminLog, nil)
	if err != nil {
		return nil, err
	}
	err = s.sqlDB.Table(TableNameAdminLog).Where(whereS, whereP...).Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}
