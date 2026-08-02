package crud

import (
	crudmodel "buildadmin-go/internal/model"
	adminauth "buildadmin-go/internal/admin/repository/auth"
	"buildadmin-go/internal/pkg/data_scope"
	cErr "buildadmin-go/internal/pkg/error"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type LogHandler struct {
	Base
	log     *zap.Logger
	crudLog *crudmodel.LogModel
	authM   *adminauth.AuthRepository
}

func NewLogHandler(log *zap.Logger, crudLog *crudmodel.LogModel, authM *adminauth.AuthRepository) *LogHandler {
	return &LogHandler{
		Base:    NewBase(crudLog),
		log:     log,
		crudLog: crudLog,
		authM:   authM,
	}
}

func (h *LogHandler) Index(ctx *gin.Context) {
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
