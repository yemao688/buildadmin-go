package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	"go-build-admin/conf"
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
	Name         string `json:"name"`
	Controller   string `json:"controller"`
	ControllerAs string `json:"controller_as"`
	DataTable    string `json:"data_table"`
	PrimaryKey   string `json:"primary_key"`
	Status       string `json:"status"`
	UpdateTime   int64  `json:"update_time"`
	CreateTime   int64  `json:"create_time"`
}

func (v DataRecycle) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{}
}

func (h *DataRecycleHandler) Add(ctx *gin.Context) {
	if ctx.Request.Method == http.MethodGet {
		Success(ctx, map[string]interface{}{
			"tables":      h.getTableList(ctx),
			"controllers": h.getRouteList(ctx),
		})
		return
	}

	var params DataRecycle
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	var data model.DataRecycle
	copier.Copy(&data, params)
	err := h.dataRecycleM.Add(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *DataRecycleHandler) Edit(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	data, err := h.dataRecycleM.GetOne(ctx, int32(id))
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	if ctx.Request.Method == http.MethodGet {
		Success(ctx, map[string]interface{}{
			"row": data,
		})
		return
	}

	var params = struct {
		IDS
		DataRecycle
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	copier.Copy(&data, params)
	err = h.dataRecycleM.Edit(ctx, data)
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

func (h *DataRecycleHandler) getRouteList(ctx *gin.Context) any {
	//TODO:
	outExcludeRoute := []string{
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

	outRoutes := map[string]string{}
	routes := GetAllRoutes()
	for _, r := range routes {
		flag := false
		for _, v := range outExcludeRoute {
			if v == r.Method {
				flag = true
				break
			}
		}
		if !flag {
			outRoutes[r.Method] = r.Method
		}
	}
	return outRoutes

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
