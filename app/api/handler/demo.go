package handler

import (
	"go-build-admin/app/admin/validate"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"go.uber.org/zap"
)

type DemoHandler struct {
	log *zap.Logger
}

func NewDemoHandler(log *zap.Logger) *DemoHandler {
	return &DemoHandler{log: log}
}

type Demo struct {
	Avatar   string    `json:"avatar" form:"avatar" binding:"omitempty"`
	Username string    `json:"username" form:"username" binding:"required,alphanum,min=2,max=15"`
	Nickname string    `json:"nickname" form:"nickname" binding:"required,alphanum"`
	Gender   int       `json:"gender" form:"gender" binding:"oneof=0 1 2"`
	Birthday time.Time `json:"birthday" form:"birthday" binding:"omitempty" time_format:"2006-01-02"` //2006-01-02T15:04:05Z 2006-01-02T15:04:05-07:08
	Motto    string    `json:"motto" form:"motto" binding:"max=255"`
}

func (v Demo) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"username.required": "username required",
		"username.alphanum": "username can only be letters and numbers",
		"username.min":      "username > 2 and username < 15",
		"username.max":      "username > 2 and username < 15",
	}
}

func (h *DemoHandler) Index(ctx *gin.Context) {
	params := Demo{}
	//可以绑定时间binding.Form
	// if err := ctx.ShouldBindWith(&params, binding.Form); err != nil {
	// 	FailByErr(ctx, validate.GetError(params, err))
	// 	return
	// }

	//可以绑定时间binding.Query
	// if err := ctx.ShouldBindWith(&params, binding.Query); err != nil {
	// 	FailByErr(ctx, validate.GetError(params, err))
	// 	return
	// }

	//json 需要使用RFC3339时间格式
	if err := ctx.ShouldBindWith(&params, binding.JSON); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	Success(ctx, params)
}
