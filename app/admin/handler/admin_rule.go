package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/tree"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type AdminRuleHandler struct {
	Base
	log        *zap.Logger
	adminRuleM *model.AdminRuleModel
	authM      *model.AuthModel
}

func NewAdminRuleHandler(log *zap.Logger, adminRuleM *model.AdminRuleModel, authM *model.AuthModel) *AdminRuleHandler {
	return &AdminRuleHandler{
		Base:       Base{currentM: adminRuleM},
		log:        log,
		adminRuleM: adminRuleM,
		authM:      authM,
	}
}

func (h *AdminRuleHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
		return
	}
	whereP := []any{}
	list, err := h.GetMenus(ctx, []string{}, whereP)
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

type AdminRule struct {
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

func (v AdminRule) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"title.required": "title required",
	}
}

func (h *AdminRuleHandler) Add(ctx *gin.Context) {
	var params AdminRule
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	var adminRule model.AdminRule
	copier.Copy(&adminRule, params)

	err := h.adminRuleM.Add(ctx, adminRule)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *AdminRuleHandler) Edit(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	adminRule, err := h.adminRuleM.GetOne(ctx, int32(id))
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	//校验数据权限
	if !h.CheckDataLimit(ctx, adminRule.ID) {
		FailByErr(ctx, cErr.BadRequest("You have no permission"))
		return
	}

	if ctx.Request.Method == http.MethodGet {
		Success(ctx, map[string]interface{}{
			"row": adminRule,
		})
		return
	}

	type AdminRuleEdit struct {
		IDS
		AdminRule
	}
	var params AdminRuleEdit
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	copier.Copy(&adminRule, params)
	err = h.adminRuleM.Edit(ctx, adminRule)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *AdminRuleHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	err := h.adminRuleM.Del(ctx, params.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *AdminRuleHandler) Select(ctx *gin.Context) (interface{}, bool) {
	if s := ctx.Request.FormValue("select"); s == "" {
		return nil, false
	}

	whereS := []string{" type in ? ", " status=? "}
	whereP := []any{[]string{"menu_dir", "menu"}, "1"}
	list, err := h.GetMenus(ctx, whereS, whereP)
	if err != nil {
		FailByErr(ctx, err)
		return nil, false
	}

	isTree := ctx.Request.FormValue("isTree")
	if isTree == "true" {
		data := tree.AssembleTree(h.AssembleChild(list).([]*AdminRuleExpend))
		return map[string]interface{}{
			"options": data,
		}, true
	}
	return nil, false
}

// 获取菜单列表
func (h *AdminRuleHandler) GetMenus(ctx *gin.Context, whereS []string, whereP []interface{}) ([]model.AdminRule, error) {
	keyword := ctx.Request.FormValue("quickSearch")
	ids, _ := h.authM.GetRuleIds(0)
	flag := false
	for _, v := range ids {
		if strings.Contains(v, "*") {
			flag = true
			break
		}
	}

	if !flag && len(ids) > 0 {
		whereS = append(whereS, " id in ? ")
		whereP = append(whereP, ids)
	}

	if keyword != "" {
		keywordArr := strings.Split(keyword, " ")
		for _, v := range keywordArr {
			whereS = append(whereS, h.adminRuleM.QuickSearchField+" LIKE ? ")
			whereP = append(whereP, "%"+strings.Replace(v, "%", "\\%", -1)+"%")
		}
	}

	list := []model.AdminRule{}
	err := h.adminRuleM.DB().Table(h.adminRuleM.TableName).Where(strings.Join(whereS, " AND "), whereP...).Order("weigh desc,id asc").Find(&list).Error
	return list, err
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
	l.Children = children.([]*AdminRuleExpend)
}

func (h *AdminRuleHandler) AssembleChild(list []model.AdminRule) interface{} {
	expendList := []*AdminRuleExpend{}
	for _, v := range list {
		temp := AdminRuleExpend{}
		copier.Copy(&temp, v)
		expendList = append(expendList, &temp)
	}
	return tree.AssembleChild(expendList)
}
