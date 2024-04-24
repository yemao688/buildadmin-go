package handler

import (
	"go-build-admin/app/admin/model"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type IndexHandler struct {
	config *conf.Configuration
	log    *zap.Logger
	authM  *model.AuthModel
}

func NewIndexHandler(config *conf.Configuration, log *zap.Logger, authM *model.AuthModel) *IndexHandler {
	return &IndexHandler{config: config, log: log, authM: authM}
}

func (h *IndexHandler) Index(ctx *gin.Context) {
	adminInfo, _ := h.authM.GetInfo(ctx, 1)
	// adminInfo, err = h.authM.IsSuperAdmin(ctx)

	menus, _ := h.authM.GetMenus(ctx, 1)
	if len(menus) == 0 {
		FailByErr(ctx, cErr.BadRequest("No background menu, please contact super administrator!"))
		return
	}
	Success(ctx, map[string]interface{}{
		"adminInfo": adminInfo,
		"menus":     menus,
		"siteConfig": map[string]interface{}{
			"siteName": "",
			"version":  "",
			"cdnUrl":   "",
			"apiUrl":   h.config.App.AppUrl,
			"upload":   "",
		},
		"terminal": map[string]interface{}{
			"installServicePort": "",
			"npmPackageManager":  "",
		},
	})
}

func (h *IndexHandler) Login(ctx *gin.Context) {
	result, err := h.authM.GetInfo(ctx, 1)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, result)
}

func (h *IndexHandler) Logout(ctx *gin.Context) {
	result, err := h.authM.GetInfo(ctx, 1)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, result)
}
