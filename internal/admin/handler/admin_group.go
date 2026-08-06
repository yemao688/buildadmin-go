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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type AdminGroupHandler struct {
	Base
	log         *zap.Logger
	adminGroupM *adminmodel.AdminGroupRepository
	adminRuleM  *adminmodel.AdminRuleRepository
	authM       *adminmodel.AuthRepository
	svc         *service.AdminGroupService
}

func NewAdminGroupHandler(log *zap.Logger, adminGroupM *adminmodel.AdminGroupRepository, adminRuleM *adminmodel.AdminRuleRepository, authM *adminmodel.AuthRepository, svc *service.AdminGroupService) *AdminGroupHandler {
	return &AdminGroupHandler{
		Base:        NewBase(adminGroupM),
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

	whereS := []string{}
	whereP := []interface{}{}
	groups, err := h.GetGroups(ctx, whereS, whereP)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	adminAuth := header.GetAdminAuth(ctx)
	result := map[string]interface{}{
		"list":   groups,
		"group":  h.authM.GetGroupIds(adminAuth.Id),
		"remark": h.GetRemark(ctx),
	}

	isTree := ctx.Request.FormValue("isTree")
	if isTree == "" || isTree == "true" {
		result["list"] = h.AssembleChild(groups)
	}
	response.Success(ctx, result)
}

type AdminGroup struct {
	Pid    int32   `json:"pid"`
	Name   string  `json:"name" binding:"required"`
	Rules  []int32 `json:"rules"`
	Status string  `json:"status"`
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
	if err := h.svc.Add(ctx.Request.Context(), adminGroup, params.Rules, adminAuth.Id, adminAuth.IsSuperAdmin); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
	requesttx.InvalidateAfterMutation(ctx, h.authM.InvalidateAll)
}

func (h *AdminGroupHandler) One(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	adminGroup, err := h.adminGroupM.GetOne(ctx.Request.Context(), int32(id))
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	if err := h.svc.CheckAuth(header.GetAdminAuth(ctx).Id, header.GetAdminAuth(ctx).IsSuperAdmin, int32(id)); err != nil {
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
	bodyBytes, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		return false
	}
	ctx.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		return false
	}

	if len(m) != 2 {
		return false
	}
	idVal, hasID := m["id"]
	if !hasID {
		return false
	}

	var fieldName string
	var fieldValue any
	for k, v := range m {
		if k != "id" {
			fieldName = k
			fieldValue = v
			break
		}
	}

	if !allowedFields[fieldName] {
		return false
	}

	id := int32(com.StrTo(fmt.Sprintf("%v", idVal)).MustInt())
	for _, validator := range validators {
		if validator == nil {
			continue
		}
		if err := validator(id, fieldName, fieldValue); err != nil {
			response.FailByErr(ctx, err)
			return true
		}
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
	if err := h.svc.Edit(ctx.Request.Context(), params.ID, adminGroup, params.Rules, adminAuth.Id, adminAuth.IsSuperAdmin); err != nil {
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

	whereS := []string{" status=? "}
	whereP := []any{"1"}
	list, err := h.GetGroups(ctx, whereS, whereP)
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

// 获取分组
func (h *AdminGroupHandler) GetGroups(ctx *gin.Context, whereS []string, whereP []interface{}) ([]*model.AdminGroup, error) {
	absoluteAuth := ctx.Request.FormValue("absoluteAuth")
	keyword := ctx.Request.FormValue("quickSearch")

	if keyword != "" {
		keywordArr := strings.Split(keyword, " ")
		for _, v := range keywordArr {
			whereS = append(whereS, h.adminGroupM.QuickSearchField+" LIKE ? ")
			whereP = append(whereP, "%"+strings.Replace(v, "%", "\\%", -1)+"%")
		}
	}

	adminAuth := header.GetAdminAuth(ctx)
	if !adminAuth.IsSuperAdmin {
		authGroups, err := h.authM.GetAllAuthGroups("allAuthAndOthers", adminAuth.Id)
		if err != nil {
			groupIds := h.authM.GetGroupIds(adminAuth.Id)
			for _, v := range groupIds {
				authGroups = append(authGroups, strconv.Itoa(int(v)))
			}
			return nil, err
		}
		// PHP 语义（Group::getGroups）：非超管始终按 authGroups 过滤。
		// absoluteAuth=0/空（普通列表）时合并自己所在分组；
		// absoluteAuth=1（管理员编辑页的授权下拉）时只显示有资格管理的分组。
		if absoluteAuth != "1" {
			for _, gid := range h.authM.GetGroupIds(adminAuth.Id) {
				authGroups = append(authGroups, strconv.Itoa(int(gid)))
			}
		}
		authGroups = util.RemoveStrDuplicates(authGroups)
		whereS = append(whereS, " id in ? ")
		whereP = append(whereP, authGroups)
	}
	list, err := h.adminGroupM.ListWhere(ctx.Request.Context(), strings.Join(whereS, " AND "), whereP...)
	if err != nil {
		return list, err
	}

	// 获取第一个权限的名称供列表显示-s
	for _, v := range list {
		if v.Rules != "" {
			if strings.Contains(v.Rules, "*") {
				v.Rules = util.Lang(ctx, "Super administrator", nil)
			} else {
				ruleIds := strings.Split(v.Rules, ",")
				num := len(ruleIds)
				if num > 0 {
					rule, err := h.adminRuleM.GetOne(ctx.Request.Context(), mustRuleID(ruleIds[0]))
					if err != nil {
						return nil, err
					}
					if num == 1 {
						v.Rules = rule.Title
					} else {
						v.Rules = rule.Title + "等 " + strconv.Itoa(num) + " 项"
					}
				}
			}
		} else {
			v.Rules = util.Lang(ctx, "no permission", nil)
		}
	}
	return list, nil
}

func mustRuleID(raw string) int32 {
	id, _ := strconv.Atoi(raw)
	return int32(id)
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
	l.Children = children.([]*AdminGroupExpend)
}

func (h *AdminGroupHandler) AssembleChild(list []*model.AdminGroup) []*AdminGroupExpend {
	expendList := []*AdminGroupExpend{}
	for _, v := range list {
		temp := AdminGroupExpend{}
		copier.Copy(&temp, v)
		expendList = append(expendList, &temp)
	}
	return tree.AssembleChild(expendList)
}
