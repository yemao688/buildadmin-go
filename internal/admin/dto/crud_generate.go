package dto

import crudmodel "buildadmin-go/internal/pkg/crudmodel"

// CrudGenerateParam 是 CRUD 设计器生成请求参数，保留原有必填绑定语义。
type CrudGenerateParam struct {
	Table  crudmodel.Table   `json:"table" binding:"required"`
	Type   string            `json:"type" binding:"required"`
	Fields []crudmodel.Field `json:"fields" binding:"required"`
}
