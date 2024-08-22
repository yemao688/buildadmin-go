package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/random"
	"go-build-admin/utils"
	"strconv"

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
	GroupID  int32  `json:"group_id"`
	Username string `json:"username" binding:"required"`
	Nickname string `json:"nickname" binding:"required"`
	Email    string `json:"email"`
	Mobile   string `json:"mobile"`
	Avatar   string `json:"avatar"`
	Gender   int32  `json:"gender"`
	Birthday string `json:"birthday" binding:"omitempty,datetime=2006-01-02"`
	JoinIP   string `json:"join_ip"`
	JoinTime int64  `json:"join_time"`
	Motto    string `json:"motto"`
	Password string `json:"password"`
	Status   string `json:"status"`
}

func (v User) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"username.min":      "username>2 and username<15",
		"username.max":      "username>2 and username<15",
		"password.password": "password invalid",
	}
}

func (h *UserHandler) Add(ctx *gin.Context) {
	var params User
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}
	if params.Password == "" {
		FailByErr(ctx, cErr.BadRequest("Please input correct password"))
		return
	}

	var user model.User
	copier.Copy(&user, params)
	birthday, _ := utils.ParseTimeShort(params.Birthday)
	user.Birthday = birthday
	user.Salt = random.Build("alnum", 16)
	user.Password = utils.EncryptPassword(params.Password, user.Salt)

	err := h.userM.Add(ctx, user)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *UserHandler) One(ctx *gin.Context) {
	value := ctx.Request.FormValue("id")
	userId := ctx.Request.FormValue("userId")
	if value == "" {
		value = userId
	}
	id := com.StrTo(value).MustInt()
	user, err := h.userM.GetOne(ctx, int32(id))
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	result, err := h.userM.DealData(ctx, &user)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	if userId != "" {
		Success(ctx, map[string]interface{}{
			"user": result,
		})
		return
	}
	Success(ctx, map[string]interface{}{
		"row": result,
	})
}

func (h *UserHandler) Edit(ctx *gin.Context) {
	var params = struct {
		IDS
		User
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}
	user, err := h.userM.GetOne(ctx, params.ID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	if params.Password != "" {
		if err := h.userM.ResetPassword(ctx, user.ID, params.Password); err != nil {
			FailByErr(ctx, err)
			return
		}
	}

	copier.Copy(&user, params)
	birthday, _ := utils.ParseTimeShort(params.Birthday)
	user.Birthday = birthday

	err = h.userM.Edit(ctx, user)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *UserHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	err := h.userM.Del(ctx, params.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *UserHandler) Select(ctx *gin.Context) (interface{}, bool) {
	if s := ctx.Request.FormValue("select"); s == "" {
		return nil, false
	}

	result, total, err := h.userM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return nil, false
	}

	list := []map[string]any{}
	for _, v := range result {
		list = append(list, map[string]any{
			"id":            v.ID,
			"nickname_text": v.Username + "(ID:+" + strconv.Itoa(int(v.ID)) + ")",
		})
	}

	return map[string]any{
		"list":   list,
		"total":  total,
		"remark": h.GetRemark(ctx),
	}, true
}
