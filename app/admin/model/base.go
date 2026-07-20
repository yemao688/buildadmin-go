package model

import (
	"context"
	"go-build-admin/app/pkg/requesttx"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BaseModel struct {
	TableName        string
	Key              string
	QuickSearchField string
	sqlDB            *gorm.DB
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

func (s *BaseModel) TableInfo() TableInfo {
	return TableInfo{
		TableName:        s.TableName,
		Key:              s.Key,
		QuickSearchField: s.QuickSearchField,
	}
}
