package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/header"
	"go-build-admin/app/pkg/tree"
	"go-build-admin/utils"
	"net/http"
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
	Pid    int32  `json:"pid"`
	Name   string `json:"name" binding:"required"`
	Rules  string `json:"rules"`
	Status string `json:"status"`
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
	h.HandleRules(ctx, &adminGroup)

	err := h.adminGroupM.Add(ctx, adminGroup)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *AdminGroupHandler) Edit(ctx *gin.Context) {
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

	if ctx.Request.Method == http.MethodGet {
		// 读取所有pid，全部从节点数组移除，父级选择状态由子级决定
		ruleIds := strings.Split(adminGroup.Rules, ",")
		pids, err := h.adminRuleM.GetRulePIds(ruleIds)
		if err != nil {
			FailByErr(ctx, err)
			return
		}

		childRuleIds := []string{}
		for _, v := range ruleIds {
			flag := false
			for _, v1 := range pids {
				if strconv.Itoa(int(v1)) == v {
					flag = true
					break
				}
			}
			if !flag {
				childRuleIds = append(childRuleIds, v)
			}
		}

		adminGroup.Rules = strings.Join(childRuleIds, ",")
		Success(ctx, map[string]interface{}{
			"row": adminGroup,
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
	groupIds := h.authM.GetGroupIds(adminAuth.Id)
	flag := false
	for _, v := range groupIds {
		if int32(id) == v {
			flag = true
			break
		}
	}

	if !flag {
		FailByErr(ctx, cErr.BadRequest("You cannot modify your own management group!"))
		return
	}

	copier.Copy(&adminGroup, params)
	h.HandleRules(ctx, &adminGroup)

	err = h.adminGroupM.Edit(ctx, adminGroup)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *AdminGroupHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	for _, v := range params.Ids {
		if err := h.CheckAuth(ctx, int32(v)); err != nil {
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
func (h *AdminGroupHandler) HandleRules(ctx *gin.Context, adminGroup *model.AdminGroup) {
	// if adminGroup.Rules != "" {
	// TODO:
	// }
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
		data := tree.AssembleTree(h.AssembleChild(list).([]*AdminGroupExpend))
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

	if !adminAuth.IsLogin {
		flag := false
		for _, v := range authGroups {
			if v == strconv.Itoa(int(groupId)) {
				flag = true
				break
			}
		}

		if !flag {
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

func (h *AdminGroupHandler) AssembleChild(list []*model.AdminGroup) interface{} {
	expendList := []*AdminGroupExpend{}
	for _, v := range list {
		temp := AdminGroupExpend{}
		copier.Copy(&temp, v)
		expendList = append(expendList, &temp)
	}
	return tree.AssembleChild(expendList)
}
