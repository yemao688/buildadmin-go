package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	"go-build-admin/utils"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type UserGroupHandler struct {
	Base
	log        *zap.Logger
	userGroupM *model.UserGroupModel
	userRuleM  *model.AdminRuleModel
	authM      *model.AuthModel
}

func NewUserGroupHandler(log *zap.Logger, userGroupM *model.UserGroupModel, userRuleM *model.AdminRuleModel, authM *model.AuthModel) *UserGroupHandler {
	return &UserGroupHandler{
		Base:       Base{currentM: userGroupM},
		log:        log,
		userGroupM: userGroupM,
		userRuleM:  userRuleM,
		authM:      authM,
	}
}

func (h *UserGroupHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
	}
	result, err := h.userGroupM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"list":   result,
		"total":  0,
		"remark": "",
	})
}

type UserGroup struct {
	Pid    int32   `json:"pid"`
	Name   string  `json:"name" binding:"required"`
	Rules  []int32 `json:"rules"`
	Status string  `json:"status"`
}

func (v UserGroup) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"name.required": "name required",
	}
}

func (h *UserGroupHandler) Add(ctx *gin.Context) {
	var params UserGroup
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	userGroup := model.UserGroup{}
	if err := copier.Copy(&userGroup, params); err != nil {
		FailByErr(ctx, err)
		return
	}
	rules, err := h.HandleRules(ctx, params.Rules)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	userGroup.Rules = rules

	err = h.userGroupM.Add(ctx, userGroup)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *UserGroupHandler) One(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	userGroup, err := h.userGroupM.GetOne(ctx, int32(id))
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	// 读取所有pid，全部从节点数组移除，父级选择状态由子级决定
	ruleIds := strings.Split(userGroup.Rules, ",")
	pids, err := h.userRuleM.GetRulePIds(ruleIds)
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
			"id":     userGroup.ID,
			"name":   userGroup.Name,
			"rules":  childRuleIds,
			"status": userGroup.Status,
		},
	})
}

func (h *UserGroupHandler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{"status": true}) {
		return
	}

	var params = struct {
		IDS
		UserGroup
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}
	userGroup, err := h.userGroupM.GetOne(ctx, params.ID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	if err := copier.Copy(&userGroup, params); err != nil {
		FailByErr(ctx, err)
		return
	}
	userGroup.Rules, err = h.HandleRules(ctx, params.Rules)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	err = h.userGroupM.Edit(ctx, userGroup)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *UserGroupHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	err := h.userGroupM.Del(ctx, params.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

// 权限节点入库前处理
func (h *UserGroupHandler) HandleRules(ctx *gin.Context, rules []int32) (string, error) {
	if len(rules) > 0 {
		list, err := h.userRuleM.List(ctx)
		if err != nil {
			return "", err
		}
		//判断是否超级管理员
		super := true
		for _, r := range list {
			if !slices.Contains(rules, r.ID) {
				super = false
				break
			}
		}
		if super {
			return "*", nil
		}

		stringRules := []string{}
		for _, v := range rules {
			stringRules = append(stringRules, strconv.Itoa(int(v)))
		}
		return strings.Join(stringRules, ","), nil
	}
	return "", nil
}
