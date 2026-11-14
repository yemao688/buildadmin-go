package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/admin/service"
	model "buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/requesttx"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/tree"
	"buildadmin-go/internal/pkg/util"
	"buildadmin-go/internal/pkg/validator"
	"fmt"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type AdminGroupHandler struct {
	log         *zap.Logger
	adminGroupM *adminmodel.AdminGroupRepository
	adminRuleM  *adminmodel.AdminRuleRepository
	authM       *adminmodel.AuthRepository
	svc         *service.AdminGroupService
}

func NewAdminGroupHandler(log *zap.Logger, adminGroupM *adminmodel.AdminGroupRepository, adminRuleM *adminmodel.AdminRuleRepository, authM *adminmodel.AuthRepository, svc *service.AdminGroupService) *AdminGroupHandler {
	return &AdminGroupHandler{
		log:         log,
		adminGroupM: adminGroupM,
		adminRuleM:  adminRuleM,
		authM:       authM,
		svc:         svc,
	}
}

func (h *AdminGroupHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		response.Success(ctx, data)
		return
	}

	groups, err := h.GetGroups(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	adminAuth := header.GetAdminAuth(ctx)
	result := map[string]interface{}{
		"list":   groups,
		"group":  h.authM.GetGroupIds(adminAuth.Id),
		"remark": remark(ctx, h.adminRuleM),
	}

	isTree := ctx.Request.FormValue("isTree")
	if isTree == "" || isTree == "true" {
		result["list"] = h.AssembleChild(groups)
	}
	response.Success(ctx, result)
}

type AdminGroup struct {
	Pid    validator.FlexInt32      `json:"pid"`
	Name   string                   `json:"name" binding:"required"`
	Rules  validator.FlexInt32Slice `json:"rules"`
	Status validator.FlexStatus     `json:"status"`
}

func (v AdminGroup) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{
		"name.required": "name required",
	}
}

func (h *AdminGroupHandler) Add(ctx *gin.Context) {
	var params AdminGroup
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	adminGroup := model.AdminGroup{}
	if err := copier.Copy(&adminGroup, params); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	adminAuth := header.GetAdminAuth(ctx)
	if err := h.svc.Add(ctx.Request.Context(), adminGroup, []int32(params.Rules), adminAuth.Id, adminAuth.IsSuperAdmin); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
	requesttx.InvalidateAfterMutation(ctx, h.authM.InvalidateAll)
}

func (h *AdminGroupHandler) One(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	adminGroup, err := h.adminGroupM.GetOne(ctx.Request.Context(), int32(id)) //nolint:gosec // G115: 后台主键参数由 com.StrTo(...).MustInt() 解析，对应数据库主键列 int(11)，业务 ID 不超过 int32 上界（管理员分组 id）
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	if err := h.svc.CheckAuth(header.GetAdminAuth(ctx).Id, header.GetAdminAuth(ctx).IsSuperAdmin, int32(id)); err != nil { //nolint:gosec // G115: 同上：分组主键列 int(11)
		response.FailByErr(ctx, err)
		return
	}

	// 读取所有pid，全部从节点数组移除，父级选择状态由子级决定
	ruleIds := strings.Split(adminGroup.Rules, ",")
	pids, err := h.adminRuleM.GetRulePIds(ctx.Request.Context(), ruleIds)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	rulesId32s, err := util.AtoiArr(ruleIds)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	childRuleIds := []int32{}
	for _, v := range rulesId32s {
		if !slices.Contains(pids, v) {
			childRuleIds = append(childRuleIds, v)
		}
	}

	response.Success(ctx, map[string]interface{}{
		"row": map[string]any{
			"id":     adminGroup.ID,
			"name":   adminGroup.Name,
			"pid":    adminGroup.Pid,
			"status": adminGroup.Status,
			"rules":  childRuleIds,
		},
	})
}

// MaybePartialEdit overrides Base.MaybePartialEdit so that switch-unit-cell
// status updates run through the service authorization chain (CheckAuth plus
// the self-group protection) instead of a raw row update by primary key.
func (h *AdminGroupHandler) MaybePartialEdit(ctx *gin.Context, allowedFields map[string]bool, validators ...PartialEditValidator) bool {
	_, id, fieldName, fieldValue, handled, failed := parsePartialEditRequest(ctx, "id", allowedFields, validators...)
	if !handled {
		return false
	}
	if failed {
		return true
	}

	if fieldName != "status" {
		return false
	}
	status, ok := fieldValue.(string)
	if !ok {
		response.FailByErr(ctx, cErr.BadRequest("status must be a string"))
		return true
	}
	adminAuth := header.GetAdminAuth(ctx)
	if err := h.svc.SwitchStatus(ctx.Request.Context(), id, status, adminAuth.Id, adminAuth.IsSuperAdmin); err != nil {
		response.FailByErr(ctx, err)
		return true
	}
	response.Success(ctx, "")
	return true
}

func (h *AdminGroupHandler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{"status": true}) {
		requesttx.InvalidateAfterMutation(ctx, h.authM.InvalidateAll)
		return
	}

	var params = struct {
		dto.IDS
		AdminGroup
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	adminGroup := model.AdminGroup{}
	if err := copier.Copy(&adminGroup, params.AdminGroup); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	adminAuth := header.GetAdminAuth(ctx)
	if err := h.svc.Edit(ctx.Request.Context(), int32(params.ID), adminGroup, []int32(params.Rules), adminAuth.Id, adminAuth.IsSuperAdmin); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
	requesttx.InvalidateAfterMutation(ctx, h.authM.InvalidateAll)
}

func (h *AdminGroupHandler) Del(ctx *gin.Context) {
	var params validator.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	adminAuth := header.GetAdminAuth(ctx)
	if err := h.svc.Del(ctx.Request.Context(), params.Ids, adminAuth.Id, adminAuth.IsSuperAdmin); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
	requesttx.InvalidateAfterMutation(ctx, h.authM.InvalidateAll)
}

func (h *AdminGroupHandler) Select(ctx *gin.Context) (interface{}, bool) {
	if s := ctx.Request.FormValue("select"); s == "" {
		return nil, false
	}

	list, err := h.listGroups(ctx, "1")
	if err != nil {
		response.FailByErr(ctx, err)
		return nil, false
	}

	isTree := ctx.Request.FormValue("isTree")
	if isTree == "" || isTree == "true" {
		data := tree.AssembleTree(tree.GetTreeArray(h.AssembleChild(list), 0, false))
		return map[string]interface{}{
			"options": data,
		}, true
	}
	return nil, false
}

// GetGroups 读取分组管理页的完整列表（tree 装配由响应层完成）。
func (h *AdminGroupHandler) GetGroups(ctx *gin.Context) ([]*model.AdminGroup, error) {
	return h.listGroups(ctx, "")
}

// listGroups 按可选状态过滤读取操作者可见的分组；授权集合运算与规则展示
// 文本补全在 service 内完成，i18n 通过 translate 回调注入。
func (h *AdminGroupHandler) listGroups(ctx *gin.Context, status string) ([]*model.AdminGroup, error) {
	adminAuth := header.GetAdminAuth(ctx)
	return h.svc.ListForOperator(ctx.Request.Context(), service.GroupListQuery{
		OperatorID:   adminAuth.Id,
		IsSuperAdmin: adminAuth.IsSuperAdmin,
		Keyword:      ctx.Request.FormValue("quickSearch"),
		AbsoluteAuth: ctx.Request.FormValue("absoluteAuth") == "1",
		Status:       status,
	}, func(messageID string) string {
		return util.Lang(ctx, messageID, nil)
	})
}

type AdminGroupExpend struct {
	model.AdminGroup
	Children []*AdminGroupExpend `json:"children"`
}

func (l *AdminGroupExpend) GetId() int               { return int(l.ID) }
func (l *AdminGroupExpend) GetPid() int              { return int(l.Pid) }
func (l *AdminGroupExpend) GetTitle() string         { return l.Name }
func (l *AdminGroupExpend) GetChildren() interface{} { return l.Children }
func (l *AdminGroupExpend) SetTitle(title string)    { l.Name = title }
func (l *AdminGroupExpend) SetChildren(children interface{}) {
	list, ok := children.([]*AdminGroupExpend)
	if !ok {
		panic(fmt.Sprintf("unexpected children type %T", children))
	}
	l.Children = list
}

func (h *AdminGroupHandler) AssembleChild(list []*model.AdminGroup) []*AdminGroupExpend {
	expendList := []*AdminGroupExpend{}
	for _, v := range list {
		temp := AdminGroupExpend{}
		_ = copier.Copy(&temp, v)
		expendList = append(expendList, &temp)
	}
	return tree.AssembleChild(expendList)
}
