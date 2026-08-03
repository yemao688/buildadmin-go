package service

import (
	"context"
	"encoding/json"
	"strings"

	securitymodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/model"
)

// SensitiveDataService 承载敏感数据规则的业务装配：controller_as 归一化
// 与 fields↔DataFields JSON 编解码。状态开关的请求识别与 table/route
// 候选列表仍属于 handler 的请求解析/响应层。
type SensitiveDataService struct {
	sensitiveDataM *securitymodel.SensitiveDataRepository
}

func NewSensitiveDataService(sensitiveDataM *securitymodel.SensitiveDataRepository) *SensitiveDataService {
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

// Add 编排敏感数据规则新增：controller_as 归一化 + fields 装配 + 落库。
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

	return s.sensitiveDataM.Add(ctx, sensitiveData)
}

// Edit 编排敏感数据规则更新：重载 + controller_as 归一化 + fields 装配 +
// 落库。
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

	return s.sensitiveDataM.Edit(ctx, data)
}
