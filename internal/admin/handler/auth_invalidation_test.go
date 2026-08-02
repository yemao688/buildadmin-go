package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go-build-admin/internal/pkg/requesttx"
	"gorm.io/gorm"
)

func newInvalidationTestContext(transactional bool, businessCode int) *gin.Context {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	request := httptest.NewRequest("POST", "/admin/test", nil)
	if transactional {
		request = request.WithContext(requesttx.Bind(request.Context(), &gorm.DB{}))
		requesttx.Stage(request.Context(), requesttx.Outcome{BusinessCode: businessCode})
	}
	ctx.Request = request
	return ctx
}

func TestInvalidateAfterMutationRegistersSuccessfulTransactionCallback(t *testing.T) {
	ctx := newInvalidationTestContext(true, 1)
	calls := 0
	invalidateAfterMutation(ctx, func() { calls++ })
	require.Zero(t, calls)

	requesttx.RunAfterCommit(ctx.Request.Context())
	require.Equal(t, 1, calls)
	requesttx.RunAfterCommit(ctx.Request.Context())
	require.Equal(t, 1, calls)
}

func TestInvalidateAfterMutationSkipsNonSuccessfulTransaction(t *testing.T) {
	for _, test := range []struct {
		name string
		code int
	}{
		{name: "zero", code: 0},
		{name: "other", code: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := newInvalidationTestContext(true, test.code)
			calls := 0
			invalidateAfterMutation(ctx, func() { calls++ })
			requesttx.RunAfterCommit(ctx.Request.Context())
			require.Zero(t, calls)
		})
	}
}

func TestInvalidateAfterMutationRunsImmediatelyWithoutTransaction(t *testing.T) {
	ctx := newInvalidationTestContext(false, 0)
	calls := 0
	invalidateAfterMutation(ctx, func() { calls++ })
	require.Equal(t, 1, calls)
}

func TestInvalidateAfterMutationHandlesNilInputs(t *testing.T) {
	calls := 0
	invalidateAfterMutation(nil, func() { calls++ })
	invalidateAfterMutation(nil, nil)
	require.Equal(t, 1, calls)
}
