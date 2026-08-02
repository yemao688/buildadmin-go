package security

import (
	"fmt"
	adminmodel "go-build-admin/internal/admin/model"
	cErr "go-build-admin/internal/pkg/error"
	"go-build-admin/internal/conf"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SecurityDataRecycle 回收规则表
type SecurityDataRecycle struct {
	ID           int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`
	Name         string `gorm:"column:name;not null;comment:规则名称" json:"name"`                       // 规则名称
	Controller   string `gorm:"column:controller;not null;comment:控制器" json:"controller"`            // 控制器
	ControllerAs string `gorm:"column:controller_as;not null;comment:控制器别名" json:"controller_as"`    // 控制器别名
	DataTable    string `gorm:"column:data_table;not null;comment:对应数据表" json:"data_table"`          // 对应数据表
	PrimaryKey   string `gorm:"column:primary_key;not null;comment:数据表主键" json:"primary_key"`        // 数据表主键
	Status       string `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"` // 状态:0=禁用,1=启用
	Connection   string `gorm:"column:connection;not null;default:'';comment:数据库连接配置标识" json:"connection"`
	UpdateTime   int64  `gorm:"autoUpdateTime;column:update_time;comment:更新时间" json:"update_time"` // 更新时间
	CreateTime   int64  `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"` // 创建时间
}

type DataRecycleModel struct {
	adminmodel.BaseModel
	config *conf.Configuration
}

func NewDataRecycleModel(sqlDB *gorm.DB, config *conf.Configuration) *DataRecycleModel {
	return &DataRecycleModel{
		BaseModel: adminmodel.NewBaseModel(config.Database.Prefix+"security_data_recycle", "id", "name", sqlDB),
		config:    config,
	}
}

func rejectDuplicateEnabledControllerAs(tx *gorm.DB, table, controllerAs string, excludeID int32) error {
	query := tx.Table(table).Where("status = ? AND controller_as = ?", "1", controllerAs)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return cErr.BadRequest("controller_as already has an enabled security rule")
	}
	return nil
}

func (s *DataRecycleModel) GetOne(ctx *gin.Context, id int32) (data SecurityDataRecycle, err error) {
	err = s.DBFor(ctx).Model(&SecurityDataRecycle{}).Where("id=?", id).First(&data).Error
	return
}

func (s *DataRecycleModel) List(ctx *gin.Context) (list []SecurityDataRecycle, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := adminmodel.QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.DBFor(ctx).Model(&SecurityDataRecycle{}).Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *DataRecycleModel) Add(ctx *gin.Context, data SecurityDataRecycle) error {
	if data.PrimaryKey == "" {
		data.PrimaryKey = "id"
	}
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		policy, err := resolveRulePolicy(tx, s.config.Database.Prefix, data.DataTable, "recycle", data.PrimaryKey, nil)
		if err != nil {
			return err
		}
		if policy.Table.PrimaryKey != data.PrimaryKey {
			return fmt.Errorf("invalid recycle rule primary key")
		}
		if data.Status == "1" {
			if err := rejectDuplicateEnabledControllerAs(tx, s.config.Database.Prefix+"security_data_recycle", data.ControllerAs, 0); err != nil {
				return err
			}
		}
		return tx.Create(&data).Error
	})
}

func (s *DataRecycleModel) Edit(ctx *gin.Context, data SecurityDataRecycle) error {
	if data.PrimaryKey == "" {
		data.PrimaryKey = "id"
	}
	updates := map[string]any{
		"name": data.Name, "controller": data.Controller, "controller_as": data.ControllerAs,
		"data_table": data.DataTable, "primary_key": data.PrimaryKey, "status": data.Status,
		"connection": data.Connection,
	}
	var result *gorm.DB
	if err := s.Transaction(ctx, func(tx *gorm.DB) error {
		_, err := resolveRulePolicy(tx, s.config.Database.Prefix, data.DataTable, "recycle", data.PrimaryKey, nil)
		if err != nil {
			return err
		}
		if data.Status == "1" {
			if err := rejectDuplicateEnabledControllerAs(tx, s.config.Database.Prefix+"security_data_recycle", data.ControllerAs, data.ID); err != nil {
				return err
			}
		}
		result = tx.Model(&SecurityDataRecycle{}).Where("id = ?", data.ID).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var visible int64
			if err := tx.Model(&SecurityDataRecycle{}).Where("id = ?", data.ID).Count(&visible).Error; err != nil {
				return err
			}
			if visible == 1 {
				return nil
			}
			return gorm.ErrRecordNotFound
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *DataRecycleModel) UpdateStatus(ctx *gin.Context, id int32, status string) error {
	var result *gorm.DB
	if err := s.Transaction(ctx, func(tx *gorm.DB) error {
		if status == "1" {
			var current SecurityDataRecycle
			if err := tx.Where("id = ?", id).First(&current).Error; err != nil {
				return err
			}
			if err := rejectDuplicateEnabledControllerAs(tx, s.config.Database.Prefix+"security_data_recycle", current.ControllerAs, id); err != nil {
				return err
			}
		}
		result = tx.Model(&SecurityDataRecycle{}).Where("id = ?", id).Update("status", status)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var visible int64
			if err := tx.Model(&SecurityDataRecycle{}).Where("id = ?", id).Count(&visible).Error; err != nil {
				return err
			}
			if visible == 1 {
				return nil
			}
			return gorm.ErrRecordNotFound
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *DataRecycleModel) Del(ctx *gin.Context, ids interface{}) error {
	values, ok := ids.([]int32)
	if !ok || len(values) == 0 {
		return fmt.Errorf("invalid security data recycle ids")
	}
	seen := make(map[int32]struct{}, len(values))
	normalized := make([]int32, 0, len(values))
	for _, id := range values {
		if id <= 0 {
			return fmt.Errorf("invalid security data recycle id %d", id)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			normalized = append(normalized, id)
		}
	}
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		var list []SecurityDataRecycle
		query := tx.Model(&SecurityDataRecycle{})
		if err := query.Where("id IN ?", normalized).Find(&list).Error; err != nil {
			return err
		}
		if len(list) != len(normalized) {
			return gorm.ErrRecordNotFound
		}
		del := query.Where("id IN ?", normalized).Delete(nil)
		if del.Error != nil {
			return del.Error
		}
		if del.RowsAffected != int64(len(normalized)) {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}
