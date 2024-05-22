package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/header"
	"go-build-admin/app/pkg/tree"
	"go-build-admin/utils"
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
	adminGroupM *model.AdminGroupModel
	adminRuleM  *model.AdminRuleModel
	authM       *model.AuthModel
}

func NewAdminGroupHandler(log *zap.Logger, adminGroupM *model.AdminGroupModel, adminRuleM *model.AdminRuleModel, authM *model.AuthModel) *AdminGroupHandler {
	return &AdminGroupHandler{
		Base:        Base{currentM: adminGroupM},
		log:         log,
		adminGroupM: adminGroupM,
		adminRuleM:  adminRuleM,
		authM:       authM,
	}
}

func (h *AdminGroupHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
		return
	}

	whereS := []string{}
	whereP := []interface{}{}
	groups, err := h.GetGroups(ctx, whereS, whereP)
	if err != nil {
		FailByErr(ctx, err)
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
	Success(ctx, result)
}

type AdminGroup struct {
	Pid    int32   `form:"pid" json:"pid"`
	Name   string  `form:"name" json:"name" binding:"required"`
	Rules  []int32 `form:"rules" json:"rules"`
	Status string  `form:"status" json:"status"`
}

func (v AdminGroup) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"name.required": "name required",
	}
}

func (h *AdminGroupHandler) Add(ctx *gin.Context) {
	var params AdminGroup
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	adminGroup := model.AdminGroup{}
	copier.Copy(&adminGroup, params)
	rules, err := h.HandleRules(ctx, params.Rules)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	adminGroup.Rules = rules

	err = h.adminGroupM.Add(ctx, adminGroup)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *AdminGroupHandler) One(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	adminGroup, err := h.adminGroupM.GetOne(ctx, int32(id))
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	if err := h.CheckAuth(ctx, int32(id)); err != nil {
		FailByErr(ctx, err)
		return
	}

	// 读取所有pid，全部从节点数组移除，父级选择状态由子级决定
	ruleIds := strings.Split(adminGroup.Rules, ",")
	pids, err := h.adminRuleM.GetRulePIds(ruleIds)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	rulesId32s, err := utils.AtoiArr(ruleIds)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	childRuleIds := []int32{}
	for _, v := range rulesId32s {
		if !slices.Contains(pids, v) {
			childRuleIds = append(childRuleIds, v)
		}
	}

	Success(ctx, map[string]interface{}{
		"row": map[string]any{
			"id":     adminGroup.ID,
			"name":   adminGroup.Name,
			"pid":    adminGroup.Pid,
			"status": adminGroup.Status,
			"rules":  childRuleIds,
		},
	})
}

func (h *AdminGroupHandler) Edit(ctx *gin.Context) {
	var params = struct {
		IDS
		AdminGroup
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	adminGroup, err := h.adminGroupM.GetOne(ctx, params.ID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	if err := h.CheckAuth(ctx, params.ID); err != nil {
		FailByErr(ctx, err)
		return
	}

	adminAuth := header.GetAdminAuth(ctx)
	groupIds := h.authM.GetGroupIds(adminAuth.Id)
	if slices.Contains(groupIds, params.ID) {
		FailByErr(ctx, cErr.BadRequest("You cannot modify your own management group!"))
		return
	}

	copier.Copy(&adminGroup, params)
	rules, err := h.HandleRules(ctx, params.Rules)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	adminGroup.Rules = rules

	err = h.adminGroupM.Edit(ctx, adminGroup)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *AdminGroupHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	for _, v := range params.Ids {
		if err := h.CheckAuth(ctx, v); err != nil {
			FailByErr(ctx, err)
			return
		}
	}

	err := h.adminGroupM.Del(ctx, params.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

// 权限节点入库前处理
func (h *AdminGroupHandler) HandleRules(ctx *gin.Context, rules []int32) (string, error) {
	if len(rules) > 0 {
		list, err := h.adminRuleM.List(ctx)
		if err != nil {
			return "", err
		}
		//判断是否超级管理员
		superAdmin := true
		for _, r := range list {
			if !slices.Contains(rules, r.ID) {
				superAdmin = false
				break
			}
		}
		if superAdmin {
			return "*", nil
		}

		stringRules := []string{}
		for _, v := range rules {
			stringRules = append(stringRules, strconv.Itoa(int(v)))
		}
		//禁止添加`拥有自己全部权限`的分组
		adminAuth := header.GetAdminAuth(ctx)
		hasRules, err := h.authM.GetRuleIds(adminAuth.Id)
		if err != nil {
			return "", err
		}
		isAll := true
		for _, v := range hasRules {
			if !slices.Contains(stringRules, v) {
				isAll = false
			}
		}
		if isAll {
			return "", cErr.BadRequest("Role group has all your rights, please contact the upper administrator to add or do not need to add!")
		}
		return strings.Join(stringRules, ","), nil
	}
	return "", nil
}

func (h *AdminGroupHandler) Select(ctx *gin.Context) (interface{}, bool) {
	if s := ctx.Request.FormValue("select"); s == "" {
		return nil, false
	}

	whereS := []string{" status=? "}
	whereP := []any{"1"}
	list, err := h.GetGroups(ctx, whereS, whereP)
	if err != nil {
		FailByErr(ctx, err)
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
		authGroups = utils.RemoveStrDuplicates(authGroups)
		if absoluteAuth == "true" {
			whereS = append(whereS, " id in ? ")
			whereP = append(whereP, authGroups)
		}
	}
	list := []*model.AdminGroup{}
	if err := h.adminGroupM.DB().Table(h.adminGroupM.TableName).Where(strings.Join(whereS, " AND "), whereP...).Find(&list).Error; err != nil {
		return list, err
	}

	// 获取第一个权限的名称供列表显示-s
	for _, v := range list {
		if v.Rules != "" {
			if strings.Contains(v.Rules, "*") {
				v.Rules = utils.Lang(ctx, "Super administrator", nil)
			} else {
				ruleIds := strings.Split(v.Rules, ",")
				num := len(ruleIds)
				if num > 0 {
					rule := model.AdminRule{}
					if err := h.adminRuleM.DB().Table(h.adminRuleM.TableName).Where(" id=? ", ruleIds[0]).First(&rule).Error; err != nil {
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
			v.Rules = utils.Lang(ctx, "no permission", nil)
		}
	}
	return list, nil
}

// 检查权限
func (h *AdminGroupHandler) CheckAuth(ctx *gin.Context, groupId int32) error {
	adminAuth := header.GetAdminAuth(ctx)
	authGroups, err := h.authM.GetAllAuthGroups("allAuthAndOthers", adminAuth.Id)
	if err != nil {
		return err
	}

	if !adminAuth.IsSuperAdmin {
		idStr := strconv.Itoa(int(groupId))
		if !slices.Contains(authGroups, idStr) {
			return cErr.BadRequest("You need to have all the permissions of the group and have additional permissions before you can operate the group~")
		}
	}
	return nil
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
