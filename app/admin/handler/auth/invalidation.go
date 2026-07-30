package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go-build-admin/app/pkg/requesttx"
)

func invalidateAfterMutation(ctx *gin.Context, invalidate func()) {
	if invalidate == nil {
		return
	}
	if ctx == nil || ctx.Request == nil {
		invalidate()
		return
	}

	requestContext := ctx.Request.Context()
	if requesttx.Active(requestContext) {
		outcome, ok := requesttx.PeekOutcome(requestContext)
		if ok && outcome.BusinessCode == 1 {
			requesttx.AfterCommit(requestContext, invalidate)
		}
		return
	}
	if ctx.Writer.Status() < http.StatusBadRequest {
		invalidate()
	}
}
