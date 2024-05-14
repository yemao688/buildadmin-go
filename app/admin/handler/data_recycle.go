package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/header"
	"go-build-admin/app/pkg/random"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type DataRecycleHandler struct {
	Base
	log          *zap.Logger
	config       *conf.Configuration
	dataRecycleM *model.DataRecycleModel
	tableM       *model.TableModel
}

func NewDataRecycleHandler(log *zap.Logger, config *conf.Configuration, dataRecycleM *model.DataRecycleModel, tableM *model.TableModel) *DataRecycleHandler {
	return &DataRecycleHandler{
		Base:         Base{currentM: dataRecycleM},
		log:          log,
		config:       config,
		dataRecycleM: dataRecycleM,
		tableM:       tableM,
	}
}

func (h *DataRecycleHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
		return
	}

	result, total, err := h.dataRecycleM.List(ctx)
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

type DataRecycle struct {
}

func (v DataRecycle) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{}
}

func (h *DataRecycleHandler) Add(ctx *gin.Context) {
	var params Admin
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	if params.Password == "" {
		FailByErr(ctx, cErr.BadRequest("password required"))
		return
	}

	adminAuth := header.GetAdminAuth(ctx)
	if len(params.GroupArr) > 0 {
		if err := h.CheckGroupAuth(ctx, params.GroupArr, adminAuth.Id); err != nil {
			FailByErr(ctx, err)
			return
		}
	}

	var admin model.Admin
	copier.Copy(&admin, params)

	admin.Salt = random.Build("alnum", 16)
	admin.Password = utils.EncryptPassword(params.Password, admin.Salt)

	err := h.dataRecycleM.Add(ctx, admin, params.GroupArr)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *DataRecycleHandler) Edit(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	admin, err := h.dataRecycleM.GetOne(ctx, int32(id))
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	//校验数据权限
	if !h.CheckDataLimit(ctx, admin.ID) {
		FailByErr(ctx, cErr.BadRequest("you have no permission"))
		return
	}

	if ctx.Request.Method == http.MethodGet {
		Success(ctx, map[string]interface{}{
			"row": admin,
		})
		return
	}

	type AdminEdit struct {
		IDS
		Admin
	}
	var params AdminEdit
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	adminAuth := header.GetAdminAuth(ctx)
	if adminAuth.Id == admin.ID && params.Status == "0" {
		FailByErr(ctx, cErr.BadRequest("please use another administrator account to disable the current account!"))
		return
	}

	copier.Copy(&admin, params)
	err = h.dataRecycleM.Edit(ctx, admin, params.GroupArr)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *DataRecycleHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	err := h.dataRecycleM.Del(ctx, params.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *DataRecycleHandler) getControllerList(ctx *gin.Context) any {
	//TODO:
	outExcludeController := []string{
		"Addon.php",
		"Ajax.php",
		"Module.php",
		"Terminal.php",
		"Dashboard.php",
		"Index.php",
		"routine/AdminInfo.php",
		"user/MoneyLog.php",
		"user/ScoreLog.php",
	}
	return outExcludeController
}

func (h *DataRecycleHandler) getTableList(ctx *gin.Context) map[string]string {
	outExcludeTable := []string{
		// 功能表
		"ba_area",
		"ba_token",
		"ba_captcha",
		"ba_admin_group_access",
		// 无删除功能
		"ba_user_money_log",
		"ba_user_score_log",
	}

	outTables := map[string]string{}
	tables := h.tableM.GetTableList()
	for name, comment := range tables {
		flag := false
		for _, v := range outExcludeTable {
			if v == name {
				flag = true
				break
			}
		}
		if !flag {
			outTables[name] = comment
		}
	}
	return outTables
}
