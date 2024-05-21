package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/conf"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type SensitiveDataHandler struct {
	Base
	log            *zap.Logger
	config         *conf.Configuration
	sensitiveDataM *model.SensitiveDataModel
}

func NewSensitiveDataHandler(log *zap.Logger, config *conf.Configuration, sensitiveDataM *model.SensitiveDataModel) *SensitiveDataHandler {
	return &SensitiveDataHandler{
		Base:           Base{currentM: sensitiveDataM},
		log:            log,
		config:         config,
		sensitiveDataM: sensitiveDataM,
	}
}

func (h *SensitiveDataHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
	}
	result, total, err := h.sensitiveDataM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"list":   result,
		"total":  total,
		"remark": "",
	})
}

type SensitiveData struct {
}

func (v SensitiveData) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{}
}

func (h *SensitiveDataHandler) Add(ctx *gin.Context) {
	var params SensitiveData
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	var sensitiveData model.SensitiveData
	copier.Copy(&sensitiveData, params)
	err := h.sensitiveDataM.Add(ctx, sensitiveData)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *SensitiveDataHandler) Edit(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	admin, err := h.sensitiveDataM.GetOne(ctx, int32(id))
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	//校验数据权限
	if !h.CheckDataLimit(ctx, admin.ID) {
		FailByErr(ctx, cErr.BadRequest("You have no permission"))
		return
	}

	if ctx.Request.Method == http.MethodGet {
		Success(ctx, map[string]interface{}{
			"row": admin,
		})
		return
	}

	type SensitiveDataEdit struct {
		IDS
		SensitiveData
	}
	var params SensitiveDataEdit
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	copier.Copy(&admin, params)
	err = h.sensitiveDataM.Edit(ctx, admin)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *SensitiveDataHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}
	err := h.sensitiveDataM.Del(ctx, params.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
