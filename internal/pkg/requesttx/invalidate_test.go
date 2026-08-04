package requesttx

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newInvalidationTestContext(transactional bool, businessCode int) *gin.Context {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	request := httptest.NewRequest("POST", "/admin/test", nil)
	if transactional {
		request = request.WithContext(Bind(request.Context(), &gorm.DB{}))
		Stage(request.Context(), Outcome{BusinessCode: businessCode})
	}
	ctx.Request = request
	return ctx
}

func TestInvalidateAfterMutationRegistersSuccessfulTransactionCallback(t *testing.T) {
	ctx := newInvalidationTestContext(true, 1)
	calls := 0
	InvalidateAfterMutation(ctx, func() { calls++ })
	require.Zero(t, calls)

	RunAfterCommit(ctx.Request.Context())
	require.Equal(t, 1, calls)
	RunAfterCommit(ctx.Request.Context())
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
			InvalidateAfterMutation(ctx, func() { calls++ })
			RunAfterCommit(ctx.Request.Context())
			require.Zero(t, calls)
		})
	}
}

func TestInvalidateAfterMutationRunsImmediatelyWithoutTransaction(t *testing.T) {
	ctx := newInvalidationTestContext(false, 0)
	calls := 0
	InvalidateAfterMutation(ctx, func() { calls++ })
	require.Equal(t, 1, calls)
}

func TestInvalidateAfterMutationHandlesNilInputs(t *testing.T) {
	calls := 0
	InvalidateAfterMutation(nil, func() { calls++ })
	InvalidateAfterMutation(nil, nil)
	require.Equal(t, 1, calls)
}
