package service

import (
	"context"
	"encoding/json"
	"strings"

	securitymodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"

	"gorm.io/gorm"
)

// SensitiveDataService 承载敏感数据规则的业务编排：controller_as 归一化、
// fields↔DataFields JSON 编解码、目标表策略校验与 controller_as 唯一性
// 约束。repo 只保留 scoped 原子原语与策略解析助手。
type SensitiveDataService struct {
	sensitiveDataM *securitymodel.SecuritySensitiveDataRepository
}

func NewSensitiveDataService(sensitiveDataM *securitymodel.SecuritySensitiveDataRepository) *SensitiveDataService {
	return &SensitiveDataService{sensitiveDataM: sensitiveDataM}
}

// Field is the plain transport-free form of the handler's SensitiveField DTO.
type Field struct {
	Name  string
	Value string
}

// SensitiveDataParams carries the plain shape of a sensitive-data rule
// create/update request.
type SensitiveDataParams struct {
	Name       string
	Controller string
	DataTable  string
	PrimaryKey string
	Fields     []Field
	Status     string
}

// NormalizeControllerAs normalizes a dot-style controller name into the
// stored slash form.
func (s *SensitiveDataService) NormalizeControllerAs(controller string) string {
	return strings.ToLower(strings.ReplaceAll(controller, ".", "/"))
}

// MarshalFields encodes the fields payload into the DataFields JSON column.
func (s *SensitiveDataService) MarshalFields(fields []Field) (string, error) {
	dateField := map[string]string{}
	for _, v := range fields {
		dateField[v.Name] = v.Value
	}
	bytesData, err := json.Marshal(dateField)
	if err != nil {
		return "", err
	}
	return string(bytesData), nil
}

// UnmarshalFields decodes the DataFields JSON column back into a map.
func (s *SensitiveDataService) UnmarshalFields(raw string) (map[string]string, error) {
	result := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Add 编排敏感数据规则新增：controller_as 归一化 + fields 装配 + 事务内
// 策略校验与唯一性约束。
func (s *SensitiveDataService) Add(ctx context.Context, p SensitiveDataParams) error {
	var sensitiveData model.SecuritySensitiveData
	sensitiveData.Name = p.Name
	sensitiveData.Controller = p.Controller
	sensitiveData.ControllerAs = s.NormalizeControllerAs(p.Controller)
	sensitiveData.DataTable = p.DataTable
	sensitiveData.PrimaryKey = p.PrimaryKey
	sensitiveData.Status = p.Status

	dataFields, err := s.MarshalFields(p.Fields)
	if err != nil {
		return err
	}
	sensitiveData.DataFields = dataFields

	return s.add(ctx, sensitiveData)
}

func (s *SensitiveDataService) add(ctx context.Context, data model.SecuritySensitiveData) error {
	if data.PrimaryKey == "" {
		data.PrimaryKey = "id"
	}
	fieldNames, err := s.fieldNames(data.DataFields)
	if err != nil {
		return err
	}
	return s.sensitiveDataM.Transaction(ctx, func(tx *gorm.DB) error {
		if _, err := s.sensitiveDataM.ResolvePolicyTx(tx, data.DataTable, "sensitive", data.PrimaryKey, fieldNames); err != nil {
			return err
		}
		if data.Status == "1" {
			n, err := s.sensitiveDataM.EnabledControllerAsCount(tx, data.ControllerAs, 0)
			if err != nil {
				return err
			}
			if n > 0 {
				return cErr.BadRequest("controller_as already has an enabled security rule")
			}
		}
		return s.sensitiveDataM.CreateTx(tx, &data)
	})
}

// Edit 编排敏感数据规则更新：重载 + controller_as 归一化 + fields 装配 +
// 事务内策略校验与唯一性约束。
func (s *SensitiveDataService) Edit(ctx context.Context, id int32, p SensitiveDataParams) error {
	data, err := s.sensitiveDataM.GetOne(ctx, id)
	if err != nil {
		return err
	}
	data.Name = p.Name
	data.Controller = p.Controller
	data.ControllerAs = s.NormalizeControllerAs(p.Controller)
	data.DataTable = p.DataTable
	data.PrimaryKey = p.PrimaryKey
	data.Status = p.Status

	dataFields, err := s.MarshalFields(p.Fields)
	if err != nil {
		return err
	}
	data.DataFields = dataFields
	if data.PrimaryKey == "" {
		data.PrimaryKey = "id"
	}
	fieldNames, err := s.fieldNames(data.DataFields)
	if err != nil {
		return err
	}
	updates := map[string]any{
		"name": data.Name, "controller": data.Controller, "controller_as": data.ControllerAs,
		"data_table": data.DataTable, "primary_key": data.PrimaryKey, "data_fields": data.DataFields,
		"status": data.Status, "connection": data.Connection,
	}
	return s.sensitiveDataM.Transaction(ctx, func(tx *gorm.DB) error {
		if _, err := s.sensitiveDataM.ResolvePolicyTx(tx, data.DataTable, "sensitive", data.PrimaryKey, fieldNames); err != nil {
			return err
		}
		if data.Status == "1" {
			n, err := s.sensitiveDataM.EnabledControllerAsCount(tx, data.ControllerAs, data.ID)
			if err != nil {
				return err
			}
			if n > 0 {
				return cErr.BadRequest("controller_as already has an enabled security rule")
			}
		}
		return s.sensitiveDataM.UpdateTx(tx, data.ID, updates)
	})
}

// UpdateStatus 编排状态开关：启用时先校验 controller_as 唯一性，再原子
// 更新状态。
func (s *SensitiveDataService) UpdateStatus(ctx context.Context, id int32, status string) error {
	return s.sensitiveDataM.Transaction(ctx, func(tx *gorm.DB) error {
		if status == "1" {
			current, err := s.sensitiveDataM.GetByIDTx(tx, id)
			if err != nil {
				return err
			}
			n, err := s.sensitiveDataM.EnabledControllerAsCount(tx, current.ControllerAs, id)
			if err != nil {
				return err
			}
			if n > 0 {
				return cErr.BadRequest("controller_as already has an enabled security rule")
			}
		}
		return s.sensitiveDataM.UpdateStatusTx(tx, id, status)
	})
}

// fieldNames extracts the sorted field names of a DataFields JSON payload.
func (s *SensitiveDataService) fieldNames(raw string) ([]string, error) {
	fields, err := s.UnmarshalFields(raw)
	if err != nil {
		return nil, err
	}
	fieldNames := make([]string, 0, len(fields))
	for field := range fields {
		fieldNames = append(fieldNames, field)
	}
	return fieldNames, nil
}
