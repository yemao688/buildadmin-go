package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/header"
	"go-build-admin/app/pkg/random"
	"go-build-admin/utils"
	"slices"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type AdminHandler struct {
	Base
	log    *zap.Logger
	adminM *model.AdminModel
	authM  *model.AuthModel
}

func NewAdminHandler(log *zap.Logger, adminM *model.AdminModel, authM *model.AuthModel) *AdminHandler {
	return &AdminHandler{
		Base:   Base{currentM: adminM},
		log:    log,
		adminM: adminM,
		authM:  authM,
	}
}

func (h *AdminHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
		return
	}

	result, total, err := h.adminM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]interface{}{
		"list":   result,
		"total":  total,
		"remark": "",
	})
}

type Admin struct {
	Username string   `json:"username" binding:"required,alphanum,min=2,max=15"`
	Nickname string   `json:"nickname" binding:"required"`
	Avatar   string   `json:"avatar" binding:""`
	Email    string   `json:"email" binding:"omitempty,email"`
	Mobile   string   `json:"mobile" binding:"omitempty,phone"`
	Password string   `json:"password" binding:"omitempty,password"`
	Motto    string   `json:"motto"`
	Status   string   `json:"status" binding:"oneof=0 1"`
	GroupArr []string `json:"group_arr" binding:"required"`
}

func (v Admin) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"username.min":      "username>2 and username<15",
		"username.max":      "username>2 and username<15",
		"email.email":       "email error",
		"mobile.phone":      "mobile error",
		"password.password": "password invalid",
	}
}

func (h *AdminHandler) Add(ctx *gin.Context) {
	var params Admin
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	if params.Password == "" {
		FailByErr(ctx, cErr.BadRequest("Please input correct password"))
		return
	}

	adminAuth := header.GetAdminAuth(ctx)
	if len(params.GroupArr) > 0 {
		if err := h.CheckGroupAuth(ctx, params.GroupArr, adminAuth.Id); err != nil {
			FailByErr(ctx, err)
			return
		}
	}

	var admin model.Admin
	copier.Copy(&admin, params)

	admin.Salt = random.Build("alnum", 16)
	admin.Password = utils.EncryptPassword(params.Password, admin.Salt)

	err := h.adminM.Add(ctx, admin, params.GroupArr)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *AdminHandler) One(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	result, err := h.adminM.GetOne(ctx, int32(id))
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	//校验数据权限
	if !h.CheckDataLimit(ctx, int32(id)) {
		FailByErr(ctx, cErr.BadRequest("You have no permission"))
		return
	}

	Success(ctx, map[string]interface{}{
		"row": result,
	})
}

func (h *AdminHandler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{"status": true}) {
		return
	}

	var params = struct {
		IDS
		Admin
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	admin, err := h.adminM.GetOne(ctx, params.ID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	adminAuth := header.GetAdminAuth(ctx)
	if adminAuth.Id == admin.ID && params.Status == "0" {
		FailByErr(ctx, cErr.BadRequest("Please use another administrator account to disable the current account!"))
		return
	}

	if params.Password != "" {
		if err := h.adminM.ResetPassword(ctx, admin.ID, params.Password); err != nil {
			FailByErr(ctx, err)
			return
		}
	}

	checkGroups := []string{}
	groupIds, _ := h.adminM.GetGroupArr(ctx, adminAuth.Id)
	for _, v := range params.GroupArr {
		for _, i := range groupIds {
			if v != strconv.Itoa(int(i)) {
				checkGroups = append(checkGroups, v)
			}
		}
	}
	if len(checkGroups) > 0 {
		if err := h.CheckGroupAuth(ctx, checkGroups, adminAuth.Id); err != nil {
			FailByErr(ctx, err)
			return
		}
	}

	copier.Copy(&admin, params)
	err = h.adminM.Edit(ctx, admin, []string{"password", "salt", "login_failure", "last_login_time", "last_login_ip"}, params.GroupArr)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *AdminHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	err := h.adminM.Del(ctx, params.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

// 检查分组权限
func (h *AdminHandler) CheckGroupAuth(ctx *gin.Context, groups []string, id int32) error {
	if ok := h.authM.IsSuperAdmin(id); ok {
		return nil
	}

	authGroups, err := h.authM.GetAllAuthGroups("allAuthAndOthers", id)
	if err != nil {
		return err
	}
	for _, v := range groups {
		if !slices.Contains(authGroups, v) {
			return cErr.BadRequest("You have no permission to add an administrator to this group!")
		}
	}
	return nil
}
