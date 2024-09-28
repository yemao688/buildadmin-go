package model

import (
	"encoding/json"
	"go-build-admin/app/admin/model/simple"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SecurityDataRecycleLog 数据回收记录表
type SecurityDataRecycleLog struct {
	ID         int32               `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`       // ID
	AdminID    int32               `gorm:"column:admin_id;not null;comment:操作管理员" json:"admin_id"`             // 操作管理员
	RecycleID  int32               `gorm:"column:recycle_id;not null;comment:回收规则ID" json:"recycle_id"`        // 回收规则ID
	Data       string              `gorm:"column:data;comment:回收的数据" json:"data"`                              // 回收的数据
	DataTable  string              `gorm:"column:data_table;not null;comment:数据表" json:"data_table"`           // 数据表
	PrimaryKey string              `gorm:"column:primary_key;not null;comment:数据表主键" json:"primary_key"`       // 数据表主键
	IsRestore  int32               `gorm:"column:is_restore;not null;comment:是否已还原:0=否,1=是" json:"is_restore"` // 是否已还原:0=否,1=是
	IP         string              `gorm:"column:ip;not null;comment:操作者IP" json:"ip"`                         // 操作者IP
	Useragent  string              `gorm:"column:useragent;not null;comment:User-Agent" json:"useragent"`      // User-Agent
	CreateTime int64               `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"`  // 创建时间
	Admin      simple.Admin        `gorm:"foreignKey:AdminID" json:"admin"`
	Recycle    SecurityDataRecycle `gorm:"foreignKey:RecycleID" json:"recycle"`
}

type DataRecycleLogModel struct {
	BaseModel
	config *conf.Configuration
}

func NewDataRecycleLogModel(sqlDB *gorm.DB, config *conf.Configuration) *DataRecycleLogModel {
	return &DataRecycleLogModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "security_data_recycle_log",
			Key:              "id",
			QuickSearchField: "recycle.name",
			DataLimit:        "",
			sqlDB:            sqlDB,
		},
		config: config,
	}
}

func (s *DataRecycleLogModel) GetOne(ctx *gin.Context, id int32) (dataRecycle SecurityDataRecycleLog, err error) {
	prefix := s.config.Database.Prefix
	err = s.sqlDB.Table(prefix+"security_data_recycle_log").
		Preload("Admin").
		Preload("Recycle").
		Joins("left join "+prefix+"admin admin on admin.id = "+prefix+"security_data_recycle_log.admin_id").
		Joins("left join "+prefix+"security_data_recycle recycle on recycle.id = "+prefix+"security_data_recycle_log.recycle_id").Where(""+prefix+"security_data_recycle_log.id=?", id).First(&dataRecycle).Error
	return
}

func (s *DataRecycleLogModel) List(ctx *gin.Context) (list []SecurityDataRecycleLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	prefix := s.config.Database.Prefix
	db := s.sqlDB.Table(prefix+"security_data_recycle_log").
		Preload("Admin").
		Preload("Recycle").
		Joins("left join "+prefix+"admin admin on admin.id = "+prefix+"security_data_recycle_log.admin_id").
		Joins("left join "+prefix+"security_data_recycle recycle on recycle.id = "+prefix+"security_data_recycle_log.recycle_id").
		Where(whereS, whereP...)

	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *DataRecycleLogModel) Restore(ctx *gin.Context, ids interface{}) error {
	list := []SecurityDataRecycleLog{}
	err := s.sqlDB.Model(&SecurityDataRecycleLog{}).Where(" id in ? ", ids).Find(&list).Error
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
		data := map[string]any{}
		if err := json.Unmarshal([]byte(v.Data), &data); err != nil {
			tx.Rollback()
			return err
		}

		if err := tx.Table(v.DataTable).Create(data).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}

	err = s.sqlDB.Model(&SecurityDataRecycleLog{}).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Delete(nil).Error
	return err
}

func (s *DataRecycleLogModel) Del(ctx *gin.Context, ids interface{}) error {
	err := s.sqlDB.Model(&SecurityDataRecycleLog{}).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Delete(nil).Error
	return err
}
