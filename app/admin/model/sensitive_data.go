package model

import (
	"encoding/json"
	"fmt"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

// SensitiveDatum 敏感数据规则表
type SecuritySensitiveData struct {
	ID           int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`
	AdminID      int32  `gorm:"column:admin_id;not null;comment:管理员ID" json:"admin_id"`           // ID
	Name         string `gorm:"column:name;not null;comment:规则名称" json:"name"`                    // 规则名称
	Controller   string `gorm:"column:controller;not null;comment:控制器" json:"controller"`         // 控制器
	ControllerAs string `gorm:"column:controller_as;not null;comment:控制器别名" json:"controller_as"` // 控制器别名
	DataTable    string `gorm:"column:data_table;not null;comment:对应数据表" json:"data_table"`       // 对应数据表
	OwnerColumn  string `gorm:"column:owner_column;not null;default:admin_id;comment:目标表所有者字段" json:"owner_column"`
	PrimaryKey   string `gorm:"column:primary_key;not null;comment:数据表主键" json:"primary_key"`        // 数据表主键
	DataFields   string `gorm:"column:data_fields;comment:敏感数据字段" json:"data_fields"`                // 敏感数据字段
	Status       string `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"` // 状态:0=禁用,1=启用
	Connection   string `gorm:"column:connection;not null;default:'';comment:数据库连接配置标识" json:"connection"`
	UpdateTime   int64  `gorm:"autoUpdateTime;column:update_time;comment:更新时间" json:"update_time"` // 更新时间
	CreateTime   int64  `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"` // 创建时间
}

type OutSensitiveData struct {
	SecuritySensitiveData
	DataFields []string `json:"data_fields"`
}

type SensitiveDataModel struct {
	BaseModel
	config   *conf.Configuration
	enforcer data_scope.Enforcer
}

func NewSensitiveDataModel(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *SensitiveDataModel {
	return &SensitiveDataModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "security_sensitive_data",
			Key:              "id",
			QuickSearchField: "controller",
			sqlDB:            sqlDB,
		},
		enforcer: enforcer,
		config:   config,
	}
}

// scoped applies the fail-closed hierarchical data-scope enforcer to
// security_sensitive_data.admin_id. Only an explicit unrestricted actor bypasses scope.
func (s *SensitiveDataModel) scoped(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if s.enforcer == nil {
			tx := db.Session(&gorm.Session{})
			_ = tx.AddError(data_scope.ErrScopedAccessDenied)
			return tx
		}
		return s.enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: s.TableName, Column: "admin_id"})
	}
}

func (s *SensitiveDataModel) DealData(ctx *gin.Context, data *SecuritySensitiveData) (*OutSensitiveData, error) {
	outSensitiveData := OutSensitiveData{}
	if err := copier.Copy(&outSensitiveData, data); err != nil {
		return nil, err
	}
	fieldData := []string{}
	result := map[string]string{}
	if err := json.Unmarshal([]byte(data.DataFields), &result); err != nil {
		return nil, err
	}
	for _, v := range result {
		fieldData = append(fieldData, v)
	}
	outSensitiveData.DataFields = fieldData
	return &outSensitiveData, nil
}

func (s *SensitiveDataModel) GetOne(ctx *gin.Context, id int32) (sensitiveData SecuritySensitiveData, err error) {
	err = s.DBFor(ctx).Model(&SecuritySensitiveData{}).Scopes(s.scoped(ctx)).Where("id=?", id).First(&sensitiveData).Error
	return
}

func (s *SensitiveDataModel) List(ctx *gin.Context) ([]*OutSensitiveData, int64, error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	var total int64 = 0
	list := []*SecuritySensitiveData{}

	db := s.DBFor(ctx).Model(&SecuritySensitiveData{}).Scopes(s.scoped(ctx)).Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error; err != nil {
		return nil, 0, err
	}

	result := []*OutSensitiveData{}
	for _, v := range list {
		outSensitiveData, err := s.DealData(ctx, v)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, outSensitiveData)
	}
	return result, total, err
}

func (s *SensitiveDataModel) Add(ctx *gin.Context, data SecuritySensitiveData) error {
	if s.enforcer == nil {
		return data_scope.ErrScopedAccessDenied
	}
	actor, err := s.enforcer.Actor(ctx)
	if err != nil {
		return err
	}
	data.AdminID = actor.AdminID
	var fields map[string]string
	if err := json.Unmarshal([]byte(data.DataFields), &fields); err != nil {
		return err
	}
	fieldNames := make([]string, 0, len(fields))
	for field := range fields {
		fieldNames = append(fieldNames, field)
	}
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		policy, err := data_scope.ResolveRulePolicy(tx, s.config.Database.Prefix, data.DataTable, "sensitive", data.PrimaryKey, fieldNames, data.OwnerColumn)
		if err != nil {
			return err
		}
		data.OwnerColumn = policy.Table.OwnerColumn
		return tx.Create(&data).Error
	})
}

func (s *SensitiveDataModel) Edit(ctx *gin.Context, data SecuritySensitiveData) error {
	if s.enforcer == nil {
		return data_scope.ErrScopedAccessDenied
	}
	if _, err := s.enforcer.Actor(ctx); err != nil {
		return err
	}
	var fields map[string]string
	if err := json.Unmarshal([]byte(data.DataFields), &fields); err != nil {
		return err
	}
	fieldNames := make([]string, 0, len(fields))
	for field := range fields {
		fieldNames = append(fieldNames, field)
	}
	updates := map[string]any{
		"name": data.Name, "controller": data.Controller, "controller_as": data.ControllerAs,
		"data_table": data.DataTable, "primary_key": data.PrimaryKey, "data_fields": data.DataFields,
		"status": data.Status, "connection": data.Connection,
	}
	var result *gorm.DB
	if err := s.Transaction(ctx, func(tx *gorm.DB) error {
		policy, err := data_scope.ResolveRulePolicy(tx, s.config.Database.Prefix, data.DataTable, "sensitive", data.PrimaryKey, fieldNames, data.OwnerColumn)
		if err != nil {
			return err
		}
		data.OwnerColumn = policy.Table.OwnerColumn
		updates["owner_column"] = data.OwnerColumn
		result = tx.Model(&SecuritySensitiveData{}).Scopes(s.scoped(ctx)).Where("id = ?", data.ID).Updates(updates)
		return result.Error
	}); err != nil {
		return err
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *SensitiveDataModel) UpdateStatus(ctx *gin.Context, id int32, status string) error {
	var result *gorm.DB
	if err := s.Transaction(ctx, func(tx *gorm.DB) error {
		result = tx.Model(&SecuritySensitiveData{}).Scopes(s.scoped(ctx)).Where("id = ?", id).Update("status", status)
		return result.Error
	}); err != nil {
		return err
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *SensitiveDataModel) Del(ctx *gin.Context, ids interface{}) error {
	values, ok := ids.([]int32)
	if !ok || len(values) == 0 {
		return fmt.Errorf("invalid security sensitive data ids")
	}
	seen := make(map[int32]struct{}, len(values))
	normalized := make([]int32, 0, len(values))
	for _, id := range values {
		if id <= 0 {
			return fmt.Errorf("invalid security sensitive data id %d", id)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			normalized = append(normalized, id)
		}
	}
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		var list []SecuritySensitiveData
		scoped := tx.Model(&SecuritySensitiveData{}).Scopes(s.scoped(ctx))
		if err := scoped.Where("id IN ?", normalized).Find(&list).Error; err != nil {
			return err
		}
		if len(list) != len(normalized) {
			return gorm.ErrRecordNotFound
		}
		del := scoped.Where("id IN ?", normalized).Delete(nil)
		if del.Error != nil {
			return del.Error
		}
		if del.RowsAffected != int64(len(normalized)) {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}
