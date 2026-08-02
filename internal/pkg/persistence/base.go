package persistence

import (
	"context"
	"go-build-admin/internal/pkg/querybuilder"
	"go-build-admin/internal/pkg/requesttx"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TableInfo is exported for compatibility with QueryBuilder.
type TableInfo = querybuilder.TableInfo

// BaseModel 提供数据表访问的基础结构，包含 DB 连接、事务支持和表元信息。
// 所有业务 Model 通过嵌入此结构获得 DB/DBFor/Transaction 等能力。
type BaseModel struct {
	TableName        string
	Key              string
	QuickSearchField string
	sqlDB            *gorm.DB
}

// NewBaseModel 供子包生成的模型构造 BaseModel；子包无法写入未导出的 sqlDB 字段。
func NewBaseModel(tableName, key, quickSearchField string, sqlDB *gorm.DB) BaseModel {
	return BaseModel{
		TableName:        tableName,
		Key:              key,
		QuickSearchField: quickSearchField,
		sqlDB:            sqlDB,
	}
}

func (s *BaseModel) DB() *gorm.DB {
	return s.sqlDB
}

func requestContext(ctx context.Context) context.Context {
	if ginCtx, ok := ctx.(*gin.Context); ok && ginCtx.Request != nil {
		return ginCtx.Request.Context()
	}
	return ctx
}

// DBFor returns the request transaction when one is active and otherwise the
// model's normal connection.
func (s *BaseModel) DBFor(ctx context.Context) *gorm.DB {
	ctx = requestContext(ctx)
	if db := requesttx.DB(ctx); db != nil {
		return db
	}
	return s.sqlDB
}

// Transaction participates in a request transaction, or starts a fallback
// transaction for this model when called outside one.
func (s *BaseModel) Transaction(ctx context.Context, fn func(*gorm.DB) error) error {
	ctx = requestContext(ctx)
	return requesttx.Transaction(requesttx.WithDB(ctx, s.sqlDB), fn)
}

func (s *BaseModel) Table() string {
	return s.TableName
}

func (s *BaseModel) PrimaryKeyName() string {
	return s.Key
}

// TableInfo 返回表元信息，供给 QueryBuilder 等工具使用。
func (s *BaseModel) TableInfo() TableInfo {
	return TableInfo{
		TableName:        s.TableName,
		Key:              s.Key,
		QuickSearchField: s.QuickSearchField,
	}
}
