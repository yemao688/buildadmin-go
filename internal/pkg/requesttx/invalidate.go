package requesttx

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// InvalidateAfterMutation 在写操作成功后执行 invalidate 回调（缓存失效等）：
// 请求处于事务中时登记到提交后回调，仅在暂存的业务码为成功时生效；非事务
// 请求在响应状态未失败时立即执行。
func InvalidateAfterMutation(ctx *gin.Context, invalidate func()) {
	if invalidate == nil {
		return
	}
	if ctx == nil || ctx.Request == nil {
		invalidate()
		return
	}

	requestContext := ctx.Request.Context()
	if Active(requestContext) {
		outcome, ok := PeekOutcome(requestContext)
		if ok && outcome.BusinessCode == 1 {
			AfterCommit(requestContext, invalidate)
		}
		return
	}
	if ctx.Writer.Status() < http.StatusBadRequest {
		invalidate()
	}
}
