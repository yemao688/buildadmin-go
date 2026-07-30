package user

import (
	adminauth "go-build-admin/app/admin/model/auth"
	adminmodel "go-build-admin/app/admin/model/user"
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/pkg/tree"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type RuleHandler struct {
	Base
	log       *zap.Logger
	userRuleM *adminmodel.RuleModel
	authM     *adminauth.AuthModel
	userAuthM MemberPermissionInvalidator
}

func NewRuleHandler(log *zap.Logger, userRuleM *adminmodel.RuleModel, authM *adminauth.AuthModel) *RuleHandler {
	return newRuleHandler(log, userRuleM, authM, nil)
}

func NewRuleHandlerWithAuth(log *zap.Logger, userRuleM *adminmodel.RuleModel, authM *adminauth.AuthModel, userAuthM MemberPermissionInvalidator) *RuleHandler {
	return newRuleHandler(log, userRuleM, authM, userAuthM)
}

func newRuleHandler(log *zap.Logger, userRuleM *adminmodel.RuleModel, authM *adminauth.AuthModel, userAuthM MemberPermissionInvalidator) *RuleHandler {
	return &RuleHandler{
		Base:      NewBase(userRuleM),
		log:       log,
		userRuleM: userRuleM,
		authM:     authM,
		userAuthM: userAuthM,
	}
}

func (h *RuleHandler) Index(ctx *gin.Context) {
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

type Rule struct {
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

func (v Rule) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"title.required": "title required",
	}
}

func (h *RuleHandler) Add(ctx *gin.Context) {
	var params Rule
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	var userRule adminmodel.Rule
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
	invalidateAfterMutation(ctx, func() {
		if h.userAuthM != nil {
			h.userAuthM.InvalidateAll()
		}
	})
}

func (h *RuleHandler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{"status": true}) {
		invalidateAfterMutation(ctx, func() {
			if h.userAuthM != nil {
				h.userAuthM.InvalidateAll()
			}
		})
		return
	}

	var params = struct {
		IDS
		Rule
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
	invalidateAfterMutation(ctx, func() {
		if h.userAuthM != nil {
			h.userAuthM.InvalidateAll()
		}
	})
}

func (h *RuleHandler) Del(ctx *gin.Context) {
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
	invalidateAfterMutation(ctx, func() {
		if h.userAuthM != nil {
			h.userAuthM.InvalidateAll()
		}
	})
}

func (h *RuleHandler) Select(ctx *gin.Context) (interface{}, bool) {
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
func (h *RuleHandler) GetRules(ctx *gin.Context, whereS []string, whereP []interface{}) ([]adminmodel.Rule, error) {
	keyword := ctx.Request.FormValue("quickSearch")
	if keyword != "" {
		keywordArr := strings.Split(keyword, " ")
		for _, v := range keywordArr {
			whereS = append(whereS, h.userRuleM.QuickSearchField+" LIKE ? ")
			whereP = append(whereP, "%"+strings.Replace(v, "%", "\\%", -1)+"%")
		}
	}

	list := []adminmodel.Rule{}
	err := h.userRuleM.DB().Table(h.userRuleM.TableName).Where(strings.Join(whereS, " AND "), whereP...).Order("weigh desc,id asc").Find(&list).Error
	return list, err
}

type RuleExpend struct {
	adminmodel.Rule
	Children []*RuleExpend `json:"children"`
}

func (l *RuleExpend) GetId() int               { return int(l.ID) }
func (l *RuleExpend) GetPid() int              { return int(l.Pid) }
func (l *RuleExpend) GetTitle() string         { return l.Title }
func (l *RuleExpend) GetChildren() interface{} { return l.Children }
func (l *RuleExpend) SetTitle(title string)    { l.Title = title }
func (l *RuleExpend) SetChildren(children interface{}) {
	l.Children = children.([]*RuleExpend)
}

func (h *RuleHandler) AssembleChild(list []adminmodel.Rule) []*RuleExpend {
	expendList := []*RuleExpend{}
	for _, v := range list {
		temp := RuleExpend{}
		copier.Copy(&temp, v)
		expendList = append(expendList, &temp)
	}
	return tree.AssembleChild(expendList)
}
