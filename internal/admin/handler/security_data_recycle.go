package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	adminmodel "buildadmin-go/internal/admin/repository"
	securitymodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/pkg/validator"
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type DataRecycleHandler struct {
	Base
	log          *zap.Logger
	config       *conf.Configuration
	dataRecycleM *securitymodel.DataRecycleRepository
	tableM       *adminmodel.TableRepository
}

func NewDataRecycleHandler(log *zap.Logger, config *conf.Configuration, dataRecycleM *securitymodel.DataRecycleRepository, tableM *adminmodel.TableRepository) *DataRecycleHandler {
	return &DataRecycleHandler{
		Base:         NewBase(dataRecycleM),
		log:          log,
		config:       config,
		dataRecycleM: dataRecycleM,
		tableM:       tableM,
	}
}

func (h *DataRecycleHandler) One(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	row, err := h.dataRecycleM.GetOne(ctx, int32(id))
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{"row": row})
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

func (v DataRecycle) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{}
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
		FailByErr(ctx, validator.GetError(params, err))
		return
	}

	params.ControllerAs = normalizeControllerAs(params.Controller)
	var data model.SecurityDataRecycle
	copier.Copy(&data, params)
	if err := h.dataRecycleM.Add(ctx, data); err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *DataRecycleHandler) Edit(ctx *gin.Context) {
	bodyBytes, _ := io.ReadAll(ctx.Request.Body)
	ctx.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err == nil && len(m) == 2 {
		_, hasID := m["id"]
		status, hasStatus := m["status"]
		if hasID && hasStatus {
			id := int32(com.StrTo(fmt.Sprintf("%v", m["id"])).MustInt())
			statusStr, _ := status.(string)
			if statusStr == "" {
				statusStr = fmt.Sprintf("%v", status)
			}
			if err := h.dataRecycleM.UpdateStatus(ctx, id, statusStr); err != nil {
				FailByErr(ctx, err)
				return
			}
			Success(ctx, "")
			return
		}
	}

	var params = struct {
		IDS
		DataRecycle
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}

	data, err := h.dataRecycleM.GetOne(ctx, params.ID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	params.ControllerAs = normalizeControllerAs(params.Controller)
	copier.Copy(&data, params)
	if err := h.dataRecycleM.Edit(ctx, data); err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *DataRecycleHandler) Del(ctx *gin.Context) {
	var params validator.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}

	if err := h.dataRecycleM.Del(ctx, params.Ids); err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *DataRecycleHandler) getRouteList(ctx *gin.Context) any {
	outExcludeRoute := []string{
		"addon",
		"ajax",
		"module",
		"terminal",
		"Dashboard",
		"Index",
		"routine.AdminInfo",
		"user.MoneyLog",
		"routine.Config",
		"auth.AdminLog",
	}

	outRoutes := map[string]string{}
	routes := GetAllRoutes()
	for _, r := range routes {
		if !strings.HasPrefix(r.Path, "/admin") {
			continue
		}
		segments := strings.Split(r.Path, "/")
		if len(segments) >= 3 {
			path := segments[2]
			if !slices.Contains(outExcludeRoute, path) {
				outRoutes[path] = path
			}
		}
	}
	return outRoutes
}

func (h *DataRecycleHandler) getTableList(ctx *gin.Context) map[string]string {
	outExcludeTable := []string{
		"area",
		"token",
		"captcha",
		"admin_group_access",
		"user_money_log",
	}

	outTables := map[string]string{}
	tables := h.tableM.GetTableList()
	for tableName, comment := range tables {
		name := strings.TrimPrefix(tableName, h.config.Database.Prefix)
		if !slices.Contains(outExcludeTable, name) {
			outTables[name] = comment
		}
	}
	return outTables
}
