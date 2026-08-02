package model_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go-build-admin/internal/pkg/persistence"
	"go-build-admin/internal/pkg/requesttx"
	"gorm.io/gorm"
)

func contextWithRequestTransaction(t *testing.T, db *gorm.DB) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", nil).WithContext(requesttx.Bind(context.Background(), db))
	return c
}

func TestBaseModelUsesRequestTransactionFromGinContext(t *testing.T) {
	requestDB := &gorm.DB{}
	modelDB := &gorm.DB{}
	s := &persistence.BaseModel{}
	*s = persistence.NewBaseModel("t", "id", "", modelDB)
	c := contextWithRequestTransaction(t, requestDB)

	require.Same(t, requestDB, s.DBFor(c))
	called := false
	require.NoError(t, s.Transaction(c, func(tx *gorm.DB) error {
		called = true
		require.Same(t, requestDB, tx)
		return nil
	}))
	require.True(t, called)
}
