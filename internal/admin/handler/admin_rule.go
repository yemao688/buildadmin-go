package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/admin/service"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/requesttx"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/tree"
	"buildadmin-go/internal/pkg/validator"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type AdminRuleHandler struct {
	Base
	log        *zap.Logger
	adminRuleM *adminmodel.AdminRuleRepository
	authM      *adminmodel.AuthRepository
	svc        *service.AdminRuleService
}

func NewAdminRuleHandler(log *zap.Logger, adminRuleM *adminmodel.AdminRuleRepository, authM *adminmodel.AuthRepository, svc *service.AdminRuleService) *AdminRuleHandler {
	return &AdminRuleHandler{
		Base:       NewBase(adminRuleM),
		log:        log,
		adminRuleM: adminRuleM,
		authM:      authM,
		svc:        svc,
	}
}

func (h *AdminRuleHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		response.Success(ctx, data)
		return
	}
	list, err := h.GetMenus(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	isTree := ctx.Request.FormValue("isTree")
	if isTree == "" || isTree == "true" {
		response.Success(ctx, map[string]interface{}{
			"list":   h.AssembleChild(list),
			"remark": "",
		})
		return
	}

	response.Success(ctx, map[string]interface{}{
		"list":   list,
		"remark": "",
	})

}

type AdminRule struct {
	Pid       int32                `json:"pid"`
	Type      string               `json:"type"`
	Title     string               `json:"title"  binding:"required"`
	Name      string               `json:"name"`
	Path      string               `json:"path"`
	Icon      string               `json:"icon"`
	MenuType  string               `json:"menu_type"`
	URL       string               `json:"url"`
	Component string               `json:"component"`
	Keepalive int32                `json:"keepalive"`
	Extend    string               `json:"extend"`
	Remark    string               `json:"remark"`
	Weigh     int32                `json:"weigh"`
	Status    validator.FlexStatus `json:"status"`
}

func (v AdminRule) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{
		"title.required": "title required",
	}
}

func (h *AdminRuleHandler) Add(ctx *gin.Context) {
	var params AdminRule
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	var adminRule model.AdminRule
	if err := copier.Copy(&adminRule, params); err != nil {
		response.FailByErr(ctx, err)
		return
	}

	err := h.svc.Add(ctx.Request.Context(), adminRule)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
	requesttx.InvalidateAfterMutation(ctx, h.authM.InvalidateAll)
}

func (h *AdminRuleHandler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{"status": true}) {
		requesttx.InvalidateAfterMutation(ctx, h.authM.InvalidateAll)
		return
	}

	var params = struct {
		dto.IDS
		AdminRule
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	adminRule, err := h.adminRuleM.GetOne(ctx, int32(params.ID))
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	if err := copier.Copy(&adminRule, params); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	err = h.svc.Edit(ctx.Request.Context(), adminRule)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
	requesttx.InvalidateAfterMutation(ctx, h.authM.InvalidateAll)
}

func (h *AdminRuleHandler) Del(ctx *gin.Context) {
	var params validator.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	err := h.svc.Del(ctx.Request.Context(), params.Ids)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
	requesttx.InvalidateAfterMutation(ctx, h.authM.InvalidateAll)
}

func (h *AdminRuleHandler) Select(ctx *gin.Context) (interface{}, bool) {
	if s := ctx.Request.FormValue("select"); s == "" {
		return nil, false
	}

	list, err := h.listMenus(ctx, []string{"menu_dir", "menu"}, "1")
	if err != nil {
		response.FailByErr(ctx, err)
		return nil, false
	}

	isTree := ctx.Request.FormValue("isTree")
	if isTree == "true" {
		data := tree.AssembleTree(tree.GetTreeArray(h.AssembleChild(list), 0, false))
		return map[string]interface{}{
			"options": data,
		}, true
	}
	return nil, false
}

// GetMenus 读取规则管理页的完整菜单列表（tree 装配由响应层完成）。
func (h *AdminRuleHandler) GetMenus(ctx *gin.Context) ([]model.AdminRule, error) {
	return h.listMenus(ctx, nil, "")
}

// listMenus 按可选类型/状态过滤读取操作者可见的规则；actor 由 handler 提取
// 后以参数传入 service，权限范围判定在 service 内完成。
func (h *AdminRuleHandler) listMenus(ctx *gin.Context, types []string, status string) ([]model.AdminRule, error) {
	adminAuth := header.GetAdminAuth(ctx)
	return h.svc.Menus(ctx.Request.Context(), adminAuth.Id, ctx.Request.FormValue("quickSearch"), types, status)
}

type AdminRuleExpend struct {
	model.AdminRule
	Children []*AdminRuleExpend `json:"children"`
}

func (l *AdminRuleExpend) GetId() int               { return int(l.ID) }
func (l *AdminRuleExpend) GetPid() int              { return int(l.Pid) }
func (l *AdminRuleExpend) GetTitle() string         { return l.Title }
func (l *AdminRuleExpend) GetChildren() interface{} { return l.Children }
func (l *AdminRuleExpend) SetTitle(title string)    { l.Title = title }
func (l *AdminRuleExpend) SetChildren(children interface{}) {
	list, ok := children.([]*AdminRuleExpend)
	if !ok {
		panic(fmt.Sprintf("unexpected children type %T", children))
	}
	l.Children = list
}

func (h *AdminRuleHandler) AssembleChild(list []model.AdminRule) []*AdminRuleExpend {
	expendList := []*AdminRuleExpend{}
	for _, v := range list {
		temp := AdminRuleExpend{}
		_ = copier.Copy(&temp, v)
		expendList = append(expendList, &temp)
	}
	return tree.AssembleChild(expendList)
}
