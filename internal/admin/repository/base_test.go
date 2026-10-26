package repository_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"buildadmin-go/internal/pkg/persistence"
	"buildadmin-go/internal/pkg/requesttx"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func contextWithRequestTransaction(t *testing.T, db *gorm.DB) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", nil).WithContext(requesttx.Bind(context.Background(), db))
	return c
}

func TestBaseModelUsesRequestTransactionFromGinContext(t *testing.T) {
	requestDB := newDryRunDB(t)
	modelDB := newDryRunDB(t)
	s := &persistence.BaseModel{}
	*s = persistence.NewBaseModel("t", "id", "", modelDB)
	c := contextWithRequestTransaction(t, requestDB)

	scoped := s.DBFor(c)
	require.NotNil(t, scoped)
	require.NotSame(t, requestDB, scoped, "DBFor must hand out an isolated handle")
	require.NotSame(t, requestDB.Statement, scoped.Statement, "DBFor must not share the request transaction statement")
	require.Same(t, requestDB.Statement.ConnPool, scoped.Statement.ConnPool, "DBFor must keep using the request transaction connection")

	called := false
	require.NoError(t, s.Transaction(c, func(tx *gorm.DB) error {
		called = true
		require.Same(t, requestDB, tx)
		return nil
	}))
	require.True(t, called)
}

func TestBaseModelDBForReturnsIndependentStatements(t *testing.T) {
	modelDB := newDryRunDB(t)
	s := &persistence.BaseModel{}
	*s = persistence.NewBaseModel("t", "id", "", modelDB)

	first := s.DBFor(context.Background())
	require.NotNil(t, first)
	require.NotSame(t, modelDB, first)
	require.NotSame(t, modelDB.Statement, first.Statement)
	require.Same(t, modelDB.Statement.ConnPool, first.Statement.ConnPool)

	// GORM's plain Session(&gorm.Session{}) shares the statement pointer, which
	// is the failure mode behind issue #9. Pollution must stay inside the
	// per-call handle returned by DBFor.
	polluted := first.Session(&gorm.Session{})
	polluted.Statement.Table = "ba_support_message"
	require.Empty(t, modelDB.Statement.Table, "shared connection statement must stay clean")
	require.Empty(t, s.DBFor(context.Background()).Statement.Table, "later DBFor calls must stay clean")
}
