package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/data_scope"
	cErr "go-build-admin/app/pkg/error"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CrudLogHandler struct {
	Base
	log     *zap.Logger
	crudLog *model.CrudLogModel
	authM   *model.AuthModel
}

func NewCrudLogHandler(log *zap.Logger, crudLog *model.CrudLogModel, authM *model.AuthModel) *CrudLogHandler {
	return &CrudLogHandler{
		Base:    Base{currentM: crudLog},
		log:     log,
		crudLog: crudLog,
		authM:   authM,
	}
}

func (h *CrudLogHandler) Index(ctx *gin.Context) {
	// 对齐 PHP 上游 crud/Log::initialize()：日志页不单独占权限节点，
	// 豁免通用规则检查后仍需持有 CRUD 主页权限（超管 * 在 Check 内自动放行）
	value, ok := ctx.Get(data_scope.ActorContextKey)
	actor, actorOK := value.(data_scope.Actor)
	if !ok || !actorOK || !h.authM.Check("crud/crud/index", actor.AdminID, "or") {
		FailByErr(ctx, cErr.ForbiddenRequest("You have no permission"))
		return
	}
	result, total, err := h.crudLog.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]interface{}{
		"list":   result,
		"total":  total,
		"remark": "",
	})
}
