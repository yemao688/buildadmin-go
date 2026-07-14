package model

import (
	"go-build-admin/app/admin/model/simple"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SecuritySensitiveDataLog 敏感数据修改记录
type SecuritySensitiveDataLog struct {
	ID            int32                 `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`         // ID
	AdminID       int32                 `gorm:"column:admin_id;not null;comment:操作管理员" json:"admin_id"`               // 操作管理员
	SensitiveID   int32                 `gorm:"column:sensitive_id;not null;comment:敏感数据规则ID" json:"sensitive_id"`    // 敏感数据规则ID
	DataTable     string                `gorm:"column:data_table;not null;comment:数据表" json:"data_table"`             // 数据表
	PrimaryKey    string                `gorm:"column:primary_key;not null;comment:数据表主键" json:"primary_key"`         // 数据表主键
	DataField     string                `gorm:"column:data_field;not null;comment:被修改字段" json:"data_field"`           // 被修改字段
	DataComment   string                `gorm:"column:data_comment;not null;comment:被修改项" json:"data_comment"`        // 被修改项
	IDValue       int32                 `gorm:"column:id_value;not null;comment:被修改项主键值" json:"id_value"`             // 被修改项主键值
	Before        string                `gorm:"column:before;comment:修改前" json:"before"`                              // 修改前
	After         string                `gorm:"column:after;comment:修改后" json:"after"`                                // 修改后
	IP            string                `gorm:"column:ip;not null;comment:操作者IP" json:"ip"`                           // 操作者IP
	Useragent     string                `gorm:"column:useragent;not null;comment:User-Agent" json:"useragent"`        // User-Agent
	IsRollback    int32                 `gorm:"column:is_rollback;not null;comment:是否已回滚:0=否,1=是" json:"is_rollback"` // 是否已回滚:0=否,1=是
	Connection    string                `gorm:"column:connection;not null;default:'';comment:数据库连接配置标识" json:"connection"`
	CreateTime    int64                 `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"` // 创建时间
	Admin         simple.Admin          `gorm:"foreignKey:AdminID" json:"admin"`
	SensitiveData SecuritySensitiveData `gorm:"foreignKey:SensitiveID" json:"sensitive"`
}

type SensitiveDataLogModel struct {
	BaseModel
	config *conf.Configuration
}

func NewSensitiveDataLogModel(sqlDB *gorm.DB, config *conf.Configuration) *SensitiveDataLogModel {
	return &SensitiveDataLogModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "security_sensitive_data_log",
			Key:              "id",
			QuickSearchField: "sensitive.name",
			DataLimit:        "",
			sqlDB:            sqlDB,
		},
		config: config,
	}
}

func (s *SensitiveDataLogModel) GetOne(ctx *gin.Context, id int32) (sensitiveData SecuritySensitiveDataLog, err error) {
	prefix := s.config.Database.Prefix
	err = s.sqlDB.Model(&SecuritySensitiveDataLog{}).
		Preload("Admin").
		Preload("SensitiveData").
		Joins("left join "+prefix+"admin admin on admin.id = "+prefix+"security_sensitive_data_log.admin_id").
		Joins("left join "+prefix+"security_sensitive_data sensitive_data on sensitive_data.id = "+prefix+"security_sensitive_data_log.sensitive_id").Where(""+prefix+"security_sensitive_data_log.id=?", id).First(&sensitiveData).Error
	return
}

func (s *SensitiveDataLogModel) List(ctx *gin.Context) (list []SecuritySensitiveDataLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	prefix := s.config.Database.Prefix
	db := s.sqlDB.Model(&SecuritySensitiveDataLog{}).
		Preload("Admin").
		Preload("SensitiveData").
		Joins("left join "+prefix+"admin admin on admin.id = "+prefix+"security_sensitive_data_log.admin_id").
		Joins("left join "+prefix+"security_sensitive_data sensitive_data on sensitive_data.id = "+prefix+"security_sensitive_data_log.sensitive_id").Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *SensitiveDataLogModel) Rollback(ctx *gin.Context, ids interface{}) error {
	list := []SecuritySensitiveDataLog{}
	err := s.sqlDB.Model(&SecuritySensitiveDataLog{}).Where(" id in ? ", ids).Find(&list).Error
	if err != nil {
		return err
	}

	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, v := range list {
		if err := tx.Table(s.config.Database.Prefix+v.DataTable).Where("`"+v.PrimaryKey+"`=?", v.IDValue).UpdateColumn("`"+v.DataField+"`", v.Before).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}

	//err = s.sqlDB.Model(&SecuritySensitiveDataLog{}).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Delete(nil).Error
	return err
}

func (s *SensitiveDataLogModel) Del(ctx *gin.Context, ids interface{}) error {
	err := s.sqlDB.Model(&SecuritySensitiveDataLog{}).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Delete(nil).Error
	return err
}
