package handler

import (
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/common/model"
	"go-build-admin/app/pkg/captcha"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/header"
	"go-build-admin/utils"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AccountHandler struct {
	log           *zap.Logger
	authM         *model.AuthModel
	userM         *model.UserModel
	userScoreLogM *model.UserScoreLogModel
	userMoneyLogM *model.UserMoneyLogModel
	captcha       *captcha.Captcha
}

func NewAccountHandler(log *zap.Logger, authM *model.AuthModel, userM *model.UserModel, userScoreLogM *model.UserScoreLogModel, userMoneyLogM *model.UserMoneyLogModel, captcha *captcha.Captcha) *AccountHandler {
	return &AccountHandler{log: log, authM: authM, userM: userM, userScoreLogM: userScoreLogM, userMoneyLogM: userMoneyLogM, captcha: captcha}
}

func (h *AccountHandler) Overview(ctx *gin.Context) {
	days, score, money := []string{}, []int{}, []int{}
	sevenDays := time.Now().AddDate(0, 0, -7)
	for i := 0; i < 7; i++ {
		days[i] = sevenDays.AddDate(0, 0, 1).Format("2006-01-02")

		num, _ := h.userScoreLogM.GetDayScore(ctx, sevenDays.AddDate(0, 0, 1), 0)
		score[i] = num

		num, _ = h.userMoneyLogM.GetDayMoney(ctx, sevenDays.AddDate(0, 0, 1), 0)
		money[i] = num
	}

	Success(ctx, map[string]any{
		"days":  days,
		"score": score,
		"money": money,
	})
}

type ProfileParam struct {
	Avatar   *string `json:"avatar" binding:"required"`
	Username *string `json:"username" binding:"required"`
	Nickname *string `json:"nickname" binding:"required"`
	Gender   *string `json:"gender" binding:"required"`
	Birthday *string `json:"birthday" binding:"required"`
	Motto    *string `json:"motto" binding:"required"`
}

func (v ProfileParam) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"type.required":    "type required",
		"captcha.required": "captcha required",
	}
}

/**
 * 会员资料 TODO: 参数验证
 */
func (h *AccountHandler) Profile(ctx *gin.Context) {
	if ctx.Request.Method == http.MethodPost {
		params := ProfileParam{}
		if err := ctx.ShouldBindJSON(&params); err != nil {
			FailByErr(ctx, validate.GetError(params, err))
			return
		}
		data := map[string]any{}
		if params.Avatar != nil {
			data["avatar"] = params.Avatar
		}
		if params.Username != nil {
			data["username"] = params.Username
		}
		if params.Nickname != nil {
			data["nickname"] = params.Nickname
		}
		if params.Gender != nil {
			data["gender"] = params.Gender
		}
		if params.Birthday != nil {
			data["birthday"] = params.Birthday
		}

		userAuth := header.GetUserAuth(ctx)
		if err := h.userM.Update(ctx, userAuth.Id, data); err != nil {
			FailByErr(ctx, err)
			return
		}
		Success(ctx, "")
		return
	}

	Success(ctx, map[string]any{
		"accountVerificationType": []string{"mobile", "email"},
	})
}

type VerificationParam struct {
	Type    string `json:"type" binding:"required"`
	Captcha string `json:"captcha" binding:"required"`
}

func (v VerificationParam) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"type.required":    "type required",
		"captcha.required": "captcha required",
	}
}

/**
 * 通过手机号或邮箱验证账户
 * 此处检查的验证码是通过 api/Ems或api/Sms发送的
 * 验证成功后，向前端返回一个 email-pass Token或着 mobile-pass Token
 * 在 changBind 方法中，通过 pass Token来确定用户已经通过了账户验证（用户未绑定邮箱/手机时通过账户密码验证）
 */
func (h *AccountHandler) Verification(ctx *gin.Context) {
	params := VerificationParam{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	userAuth := header.GetUserAuth(ctx)
	user, err := h.userM.GetOne(ctx, userAuth.Id)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	id := ""
	if params.Type == "email" {
		id = user.Email + "user_" + params.Type + "_verify"
	} else {
		id = user.Mobile + "user_" + params.Type + "_verify"
	}

	if !h.captcha.Check(params.Captcha, id) {
		FailByErr(ctx, cErr.BadRequest("Please enter the correct verification code"))
		return
	}
	verificationToken := h.authM.SetVerificationToken(params.Type+"-pass", user.ID)
	Success(ctx, map[string]any{
		"type":                     params.Type,
		"accountVerificationToken": verificationToken,
	})
}

type ChangeBindParam struct {
	Type                     string `json:"type"`
	Captcha                  string `json:"captcha"`
	Email                    string `json:"email"`
	Mobile                   string `json:"mobile"`
	AccountVerificationToken string `json:"accountVerificationToken"`
	Password                 string `json:"password"`
}

func (v ChangeBindParam) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{}
}

/**
 * 修改绑定信息（手机号、邮箱） TODO: 参数验证
 * 通过 pass Token来确定用户已经通过了账户验证，也就是以上的 verification 方法，同时用户未绑定邮箱/手机时通过账户密码验证
 */
func (h *AccountHandler) ChangeBind(ctx *gin.Context) {
	params := ChangeBindParam{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	userAuth := header.GetUserAuth(ctx)
	user, err := h.userM.GetOne(ctx, userAuth.Id)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	id := ""
	if params.Type == "email" && params.Email != "" {
		id = params.Email
		if !h.authM.VerificationToken(params.AccountVerificationToken, params.Type+"-pass", userAuth.Id) {
			FailByErr(ctx, cErr.BadRequest("You need to verify your account before modifying the binding information"))
			return
		}
	} else if params.Type == "mobile" && params.Mobile != "" {
		id = params.Mobile
		if !h.authM.VerificationToken(params.AccountVerificationToken, params.Type+"-pass", userAuth.Id) {
			FailByErr(ctx, cErr.BadRequest("You need to verify your account before modifying the binding information"))
			return
		}
	} else {
		id = params.Password
		if user.Password != utils.EncryptPassword(params.Password, user.Salt) {
			FailByErr(ctx, cErr.BadRequest("Old password error"))
			return
		}
	}

	// 检查验证码
	if !h.captcha.Check(params.Captcha, id+"user_change_"+params.Type) {
		FailByErr(ctx, cErr.BadRequest("Please enter the correct verification code"))
		return
	}

	if params.Type == "email" {
		h.userM.Update(ctx, user.ID, map[string]any{
			"email": params.Email,
		})
	} else if params.Type == "mobile" {
		h.userM.Update(ctx, user.ID, map[string]any{
			"mobile": params.Mobile,
		})
	}
	Success(ctx, "")
}

type ChangePasswordParam struct {
	OldPassword string `json:"oldPassword" binding:"required,password"`
	NewPassword string `json:"newPassword" binding:"required,password"`
}

func (v ChangePasswordParam) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"oldPassword.required": "oldPassword required",
		"newPassword.required": "newPassword required",
	}
}

func (h *AccountHandler) ChangePassword(ctx *gin.Context) {
	params := ChangePasswordParam{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	userAuth := header.GetUserAuth(ctx)
	user, err := h.userM.GetOne(ctx, userAuth.Id)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	if user.Password != utils.EncryptPassword(params.OldPassword, user.Salt) {
		FailByErr(ctx, cErr.BadRequest("Old password error"))
		return
	}

	if err := h.userM.ResetPassword(ctx, userAuth.Id, params.NewPassword); err != nil {
		FailByErr(ctx, err)
		return
	}
	h.authM.Logout(ctx, "")
	Success(ctx, "")
}

// 积分日志
func (h *AccountHandler) Integral(ctx *gin.Context) {
	userId := 0
	result, total, err := h.userScoreLogM.List(ctx, userId)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"list":  result,
		"total": total,
	})
}

// 余额日志
func (h *AccountHandler) Balance(ctx *gin.Context) {
	userId := 0
	result, total, err := h.userMoneyLogM.List(ctx, userId)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"list":  result,
		"total": total,
	})
}

type RetrieveParam struct {
	Type     string `json:"type"`
	Account  string `json:"account" binding:"required"`
	Captcha  string `json:"captcha" binding:"required"`
	Password string `json:"password" binding:"required,password"`
}

func (v RetrieveParam) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"account.required":  "account required",
		"captcha.required":  "captcha required",
		"password.required": "password required",
	}
}

// 找回密码
func (h *AccountHandler) RetrievePassword(ctx *gin.Context) {
	params := RetrieveParam{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	var err error
	user := model.User{}
	if params.Type == "email" {
		user, err = h.userM.GetOneByEmail(ctx, "email")
	} else {
		user, err = h.userM.GetOneByMobile(ctx, "mobile")
	}

	if err != nil {
		FailByErr(ctx, cErr.NotFound("Account does not exist~"))
		return
	}

	if !h.captcha.Check(params.Captcha, params.Account+"user_retrieve_pwd") {
		FailByErr(ctx, cErr.BadRequest("Please enter the correct verification code"))
		return
	}

	if err := h.userM.ResetPassword(ctx, user.ID, params.Password); err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
