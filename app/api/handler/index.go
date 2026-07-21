package handler

import (
	adminModel "go-build-admin/app/admin/model"
	"go-build-admin/app/common/model"
	"go-build-admin/app/common/model/country"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/tree"
	"go-build-admin/conf"
	"go-build-admin/utils"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type IndexHandler struct {
	log     *zap.Logger
	authM   *model.AuthModel
	config  *conf.Configuration
	configM *adminModel.ConfigModel
	country *country.Service
}

func NewIndexHandler(log *zap.Logger, authM *model.AuthModel, config *conf.Configuration, configM *adminModel.ConfigModel, countryService *country.Service) *IndexHandler {
	return &IndexHandler{log: log, authM: authM, config: config, configM: configM, country: countryService}
}

// 前台和会员中心的初始化请求
func (h *IndexHandler) Index(ctx *gin.Context) {
	rules := []model.Rule{}
	menus := []model.Rule{}
	userInfo := map[string]any{}
	token, isLogin := h.authM.IsLogin(ctx)
	if isLogin {
		userMenus, err := h.authM.GetMenus(ctx, token.UserID)
		if err != nil {
			FailByErr(ctx, err)
			return
		}
		for _, v := range userMenus {
			if v.Type == "menu_dir" {
				menus = append(menus, v)
			} else if v.Type != "menu" {
				rules = append(rules, v)
			}
		}

		user, err := h.authM.GetInfo(ctx, token.UserID)
		if err != nil {
			FailByErr(ctx, err)
			return
		}
		userInfo = h.authM.FilterData(user)
		userInfo["token"] = token.Token
		userInfo["refresh_token"] = ""
	} else {
		// 若是从前台会员中心内发出的请求，要求必须登录，否则会员中心异常
		requiredLogin := ctx.Query("requiredLogin")
		if requiredLogin == "1" {
			FailByErrWithData(ctx, cErr.BadRequest("Please login first", 303), map[string]string{
				"type": "need login",
			})
			return
		}
	}
	basicConfig, err := h.configM.GetKVByGroup(ctx, "basics")
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	languages, err := h.country.EnabledLanguages(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	currencies, err := h.country.EnabledCurrencies(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	Success(ctx, map[string]any{
		"site": map[string]any{
			"siteName":     basicConfig["site_name"],
			"recordNumber": basicConfig["record_number"],
			"version":      basicConfig["version"],
			"cdnUrl":       utils.FullUrl("", h.config.App.CdnUrl, utils.GetBaseURL(ctx), ""),
			"upload":       h.config.Upload,
		},
		"openMemberCenter": true,
		"userInfo":         userInfo,
		"rules":            rules,
		"menus":            menus,
		"language":         languages,
		"currency":         currencies,
	})
}

type UserRuleExpend struct {
	adminModel.UserRule
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

func (h *IndexHandler) AssembleChild(list []adminModel.UserRule) []*UserRuleExpend {
	expendList := []*UserRuleExpend{}
	for _, v := range list {
		temp := UserRuleExpend{}
		copier.Copy(&temp, v)
		expendList = append(expendList, &temp)
	}
	return tree.AssembleChild(expendList)
}
