package model

import (
	cErr "go-build-admin/app/pkg/error"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const TableNameSecurityDataRecycle = "ba_security_data_recycle"

// SecurityDataRecycle 回收规则表
type SecurityDataRecycle struct {
	ID           int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`        // ID
	Name         string `gorm:"column:name;not null;comment:规则名称" json:"name"`                       // 规则名称
	Controller   string `gorm:"column:controller;not null;comment:控制器" json:"controller"`            // 控制器
	ControllerAs string `gorm:"column:controller_as;not null;comment:控制器别名" json:"controller_as"`    // 控制器别名
	DataTable    string `gorm:"column:data_table;not null;comment:对应数据表" json:"data_table"`          // 对应数据表
	PrimaryKey   string `gorm:"column:primary_key;not null;comment:数据表主键" json:"primary_key"`        // 数据表主键
	Status       string `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"` // 状态:0=禁用,1=启用
	UpdateTime   int64  `gorm:"column:update_time;comment:更新时间" json:"update_time"`                  // 更新时间
	CreateTime   int64  `gorm:"column:create_time;comment:创建时间" json:"create_time"`                  // 创建时间
}

type DataRecycleModel struct {
	BaseModel
	sqlDB *gorm.DB
}

func NewDataRecycleModel(sqlDB *gorm.DB) *DataRecycleModel {
	return &DataRecycleModel{
		BaseModel: BaseModel{
			TableName:        TableNameSecurityDataRecycle,
			Key:              "id",
			QuickSearchField: "title",
			DataLimit:        "",
		},
		sqlDB: sqlDB}
}

func (s *DataRecycleModel) GetOne(ctx *gin.Context, id int32) (dataRecycle SecurityDataRecycle, err error) {
	err = s.sqlDB.Table(s.TableName).Where("id=?", id).First(&dataRecycle).Error
	return
}

func (s *DataRecycleModel) List(ctx *gin.Context) (list []SecurityDataRecycle, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, err
	}
	err = s.sqlDB.Table(s.TableName).Where(whereS, whereP...).Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *DataRecycleModel) GetRemark(ctx *gin.Context) string {
	var dataRecycle SecurityDataRecycle
	name := ctx.Request.URL.Path
	err := s.sqlDB.Where("name = ?", name).First(&dataRecycle).Error
	if err != nil {
		return ""
	}
	return dataRecycle.Remark
}

func (s *DataRecycleModel) Add(ctx *gin.Context, dataRecycle SecurityDataRecycle) error {
	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Table(s.TableName).Create(&dataRecycle).Error; err != nil {
		tx.Rollback()
		return err

	}
	return tx.Commit().Error
}

func (s *DataRecycleModel) Edit(ctx *gin.Context, dataRecycle SecurityDataRecycle) error {
	parent := SecurityDataRecycle{}
	if err := s.sqlDB.Table(s.TableName).Where("id=?", dataRecycle.Pid).First(&parent).Error; err != nil {
		return err
	}

	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if parent.Pid == dataRecycle.ID {
		if err := tx.Table(s.TableName).Where("id=?", parent.ID).Update("pid", 0).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Table(s.TableName).Save(&dataRecycle).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (s *DataRecycleModel) Del(ctx *gin.Context, ids []int64) error {
	var subIds []int64
	if err := s.sqlDB.Table(s.TableName).Where(" pid in ? ", ids).Pluck("id", &subIds).Error; err != nil {
		return err
	}

	for _, v := range subIds {
		flag := false
		for _, v1 := range ids {
			if v == v1 {
				flag = true
				break
			}
		}
		if !flag {
			return cErr.BadRequest("please delete the child element first, or use batch deletion")
		}
	}

	err := s.sqlDB.Table(s.TableName).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Delete(nil).Error
	return err
}

func (s *DataRecycleModel) GetRulePIds(ids []string) ([]int32, error) {
	pids := []int32{}
	err := s.sqlDB.Table(s.TableName).Where("id in ?", ids).Pluck("pid", &pids).Error
	return pids, err
}
