package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/tree"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type UserRuleHandler struct {
	Base
	log       *zap.Logger
	userRuleM *model.UserRuleModel
	authM     *model.AuthModel
}

func NewUserRuleHandler(log *zap.Logger, userRuleM *model.UserRuleModel, authM *model.AuthModel) *UserRuleHandler {
	return &UserRuleHandler{
		Base:      Base{currentM: userRuleM},
		log:       log,
		userRuleM: userRuleM,
		authM:     authM,
	}
}

func (h *UserRuleHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
		return
	}
	whereP := []any{}
	list, err := h.GetRules(ctx, []string{}, whereP)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	isTree := ctx.Request.FormValue("isTree")
	if isTree == "" || isTree == "true" {
		Success(ctx, map[string]interface{}{
			"list":   h.AssembleChild(list),
			"remark": "",
		})
		return
	}

	Success(ctx, map[string]interface{}{
		"list":   list,
		"remark": "",
	})

}

type UserRule struct {
	Pid       int32  `json:"pid"`
	Type      string `json:"type"`
	Title     string `json:"title"  binding:"required"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Icon      string `json:"icon"`
	MenuType  string `json:"menu_type"`
	URL       string `json:"url"`
	Component string `json:"component"`
	Keepalive int32  `json:"keepalive"`
	Extend    string `json:"extend"`
	Remark    string `json:"remark"`
	Weigh     int32  `json:"weigh"`
	Status    string `json:"status"`
}

func (v UserRule) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"title.required": "title required",
	}
}

func (h *UserRuleHandler) Add(ctx *gin.Context) {
	var params UserRule
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	var userRule model.UserRule
	if err := copier.Copy(&userRule, params); err != nil {
		FailByErr(ctx, err)
		return
	}

	err := h.userRuleM.Add(ctx, userRule)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *UserRuleHandler) Edit(ctx *gin.Context) {
	var params = struct {
		IDS
		UserRule
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}
	userRule, err := h.userRuleM.GetOne(ctx, params.ID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	//校验数据权限
	if !h.CheckDataLimit(ctx, userRule.ID) {
		FailByErr(ctx, cErr.BadRequest("You have no permission"))
		return
	}

	if err := copier.Copy(&userRule, params); err != nil {
		FailByErr(ctx, err)
		return
	}
	err = h.userRuleM.Edit(ctx, userRule)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *UserRuleHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	err := h.userRuleM.Del(ctx, params.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *UserRuleHandler) Select(ctx *gin.Context) (interface{}, bool) {
	if s := ctx.Request.FormValue("select"); s == "" {
		return nil, false
	}

	whereS := []string{" type in ? ", " status=? "}
	whereP := []any{[]string{"menu_dir", "menu"}, "1"}
	list, err := h.GetRules(ctx, whereS, whereP)
	if err != nil {
		FailByErr(ctx, err)
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

// 获取菜单列表
func (h *UserRuleHandler) GetRules(ctx *gin.Context, whereS []string, whereP []interface{}) ([]model.UserRule, error) {
	keyword := ctx.Request.FormValue("quickSearch")
	if keyword != "" {
		keywordArr := strings.Split(keyword, " ")
		for _, v := range keywordArr {
			whereS = append(whereS, h.userRuleM.QuickSearchField+" LIKE ? ")
			whereP = append(whereP, "%"+strings.Replace(v, "%", "\\%", -1)+"%")
		}
	}

	list := []model.UserRule{}
	err := h.userRuleM.DB().Table(h.userRuleM.TableName).Where(strings.Join(whereS, " AND "), whereP...).Order("weigh desc,id asc").Find(&list).Error
	return list, err
}

type UserRuleExpend struct {
	model.UserRule
	Children []*UserRuleExpend `json:"children"`
}

func (l *UserRuleExpend) GetId() int               { return int(l.ID) }
func (l *UserRuleExpend) GetPid() int              { return int(l.Pid) }
func (l *UserRuleExpend) GetTitle() string         { return l.Title }
func (l *UserRuleExpend) GetChildren() interface{} { return l.Children }
func (l *UserRuleExpend) SetTitle(title string)    { l.Title = title }
func (l *UserRuleExpend) SetChildren(children interface{}) {
	l.Children = children.([]*UserRuleExpend)
}

func (h *UserRuleHandler) AssembleChild(list []model.UserRule) []*UserRuleExpend {
	expendList := []*UserRuleExpend{}
	for _, v := range list {
		temp := UserRuleExpend{}
		copier.Copy(&temp, v)
		expendList = append(expendList, &temp)
	}
	return tree.AssembleChild(expendList)
}
