package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	"go-build-admin/conf"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
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
		"remark": h.GetRemark(ctx),
	})
}

type DataRecycle struct {
	Name         string `json:"name"`
	Controller   string `json:"controller"`
	ControllerAs string `json:"controller_as"`
	DataTable    string `json:"data_table"`
	PrimaryKey   string `json:"primary_key"`
	Status       string `json:"status"`
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

	var data model.SecurityDataRecycle
	copier.Copy(&data, params)
	err := h.dataRecycleM.Add(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *DataRecycleHandler) Edit(ctx *gin.Context) {
	var params = struct {
		IDS
		DataRecycle
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	data, err := h.dataRecycleM.GetOne(ctx, params.ID)
	if err != nil {
		FailByErr(ctx, err)
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
	if err := ctx.ShouldBindQuery(&params); err != nil {
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
	outExcludeRoute := []string{
		"admin/addon",
		"admin/ajax",
		"admin/module",
		"admin/terminal",
		"admin/Dashboard",
		"admin/Index",
		"admin/routine.AdminInfo",
		"admin/user.MoneyLog",
		"admin/user.ScoreLog",
	}

	outRoutes := map[string]string{}
	routes := GetAllRoutes()
	for _, r := range routes {
		for _, v := range outExcludeRoute {
			if !strings.HasPrefix(r.Path, "/admin") {
				continue
			}
			segments := strings.Split(r.Path, "/")
			if len(segments) >= 3 {
				path := segments[1] + "/" + segments[2]
				if path != v {
					outRoutes[path] = path
				}
			}
		}
	}
	return outRoutes

}

func (h *DataRecycleHandler) getTableList(ctx *gin.Context) map[string]string {
	outExcludeTable := []string{
		// 功能表
		"area",
		"token",
		"captcha",
		"admin_group_access",
		// 无删除功能
		"user_money_log",
		"user_score_log",
	}

	outTables := map[string]string{}
	tables := h.tableM.GetTableList()
	for name, comment := range tables {
		if !slices.Contains(outExcludeTable, strings.TrimLeft(name, h.config.Database.Prefix)) {
			outTables[name] = comment
		}
	}
	return outTables
}
