package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/random"
	"go-build-admin/utils"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type UserHandler struct {
	Base
	log   *zap.Logger
	userM *model.UserModel
}

func NewUserHandler(log *zap.Logger, userM *model.UserModel) *UserHandler {
	return &UserHandler{
		Base:  Base{currentM: userM},
		log:   log,
		userM: userM,
	}
}

func (h *UserHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
		return
	}

	result, total, err := h.userM.List(ctx)
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

type User struct {
	GroupID       int32     `json:"group_id"`
	Username      string    `json:"username"`
	Nickname      string    `json:"nickname"`
	Email         string    `json:"email"`
	Mobile        string    `json:"mobile"`
	Avatar        string    `json:"avatar"`
	Gender        int32     `json:"gender"`
	Birthday      time.Time `json:"birthday"`
	Money         int32     `json:"money"`
	Score         int32     `json:"score"`
	LastLoginTime int64     `json:"last_login_time"`
	LastLoginIP   string    `json:"last_login_ip"`
	LoginFailure  int32     `json:"login_failure"`
	JoinIP        string    `json:"join_ip"`
	JoinTime      int64     `json:"join_time"`
	Motto         string    `json:"motto"`
	Password      string    `json:"password"`
	Salt          string    `json:"salt"`
	RongToken     string    `json:"rong_token"`
	Openid        string    `json:"openid"`
	OpenType      string    `json:"open_type"`
	Online        int32     `json:"online"`
	Intention     int32     `json:"intention"`
	Status        string    `json:"status"`
}

func (h *UserHandler) Add(ctx *gin.Context) {
	var params User
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	if params.Password == "" {
		FailByErr(ctx, cErr.BadRequest("password required"))
		return
	}

	var user model.User
	copier.Copy(&user, params)

	user.Salt = random.Build("alnum", 16)
	user.Password = utils.EncryptPassword(params.Password, user.Salt)

	err := h.userM.Add(ctx, user)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *UserHandler) Edit(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	user, err := h.userM.GetOne(ctx, int32(id))
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	if ctx.Request.Method == http.MethodGet {
		user.Password = ""
		user.Salt = ""
		Success(ctx, map[string]interface{}{
			"row": user,
		})
		return
	}

	type UserEdit struct {
		IDS
		User
	}
	var params UserEdit
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	if params.Password != "" {
		if err := h.userM.ResetPassword(ctx, user.ID, params.Password); err != nil {
			FailByErr(ctx, err)
			return
		}
	}

	copier.Copy(&user, params)
	err = h.userM.Edit(ctx, "create_time, update_time, password, salt, login_failure, last_login_time", user)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
