package handler

import (
	"fmt"
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
	days, score, money := []string{}, []int{}, []string{}

	userAuth := header.GetUserAuth(ctx)
	sevenDays := time.Now().AddDate(0, 0, -6)
	for i := 0; i < 7; i++ {
		days = append(days, sevenDays.AddDate(0, 0, i).Format("2006-01-02"))

		scoreNum, _ := h.userScoreLogM.GetDayScore(ctx, sevenDays.AddDate(0, 0, i), userAuth.Id)
		score = append(score, scoreNum)

		moneyNum, _ := h.userMoneyLogM.GetDayMoney(ctx, sevenDays.AddDate(0, 0, i), userAuth.Id)
		money = append(money, fmt.Sprintf("%.2f", float64(moneyNum/100)))
	}

	Success(ctx, map[string]any{
		"days":  days,
		"score": score,
		"money": money,
	})
}

type ProfileParam struct {
	Avatar   string `json:"avatar" binding:"omitempty"`
	Username string `json:"username" binding:"required,alphanum,min=2,max=15"`
	Nickname string `json:"nickname" binding:"required,alphanum"`
	Gender   int    `json:"gender" binding:"oneof=0 1 2"`
	Birthday string `json:"birthday" binding:"omitempty,datetime=2006-01-02"`
	Motto    string `json:"motto" binding:"max=255"`
}

func (v ProfileParam) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"username.required": "username required",
		"username.alphanum": "username can only be letters and numbers",
		"username.min":      "username > 2 and username < 15",
		"username.max":      "username > 2 and username < 15",
	}
}

/**
 * 会员资料
 */
func (h *AccountHandler) Profile(ctx *gin.Context) {
	if ctx.Request.Method == http.MethodPost {
		params := ProfileParam{}
		if err := ctx.ShouldBindJSON(&params); err != nil {
			FailByErr(ctx, validate.GetError(params, err))
			return
		}

		userAuth := header.GetUserAuth(ctx)
		_, err := h.userM.IsExist(ctx, "username", params.Username, userAuth.Id)
		if err == nil {
			FailByErr(ctx, cErr.BadRequest("Account exist"))
			return
		}

		data := map[string]any{}
		data["avatar"] = params.Avatar
		data["username"] = params.Username
		data["nickname"] = params.Nickname
		data["gender"] = params.Gender
		data["birthday"] = (params.Birthday)[:10]
		data["motto"] = params.Motto

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
	Type                     string `json:"type" binding:"required"`
	Captcha                  string `json:"captcha" binding:"required"`
	Email                    string `json:"email" binding:"required_if=Type email,omitempty,email"`
	Mobile                   string `json:"mobile" binding:"required_if=Type mobile,omitempty,phone"`
	AccountVerificationToken string `json:"accountVerificationToken"`
	Password                 string `json:"password"`
}

func (v ChangeBindParam) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"type.required": "type required",
		"email.email":   "email invalid",
	}
}

/**
 * 修改绑定信息（手机号、邮箱）
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

	verType := ""
	if params.Type == "email" && user.Email != "" {
		verType = params.Email
		if !h.authM.VerificationToken(params.AccountVerificationToken, params.Type+"-pass", userAuth.Id) {
			FailByErr(ctx, cErr.BadRequest("You need to verify your account before modifying the binding information"))
			return
		}

	} else if params.Type == "mobile" && user.Mobile != "" {
		verType = params.Mobile
		if !h.authM.VerificationToken(params.AccountVerificationToken, params.Type+"-pass", userAuth.Id) {
			FailByErr(ctx, cErr.BadRequest("You need to verify your account before modifying the binding information"))
			return
		}

	} else {
		verType = params.Password
		if user.Password != utils.EncryptPassword(params.Password, user.Salt) {
			FailByErr(ctx, cErr.BadRequest("Old password error"))
			return
		}
	}

	// 检查验证码
	if !h.captcha.Check(params.Captcha, verType+"user_change_"+params.Type) {
		FailByErr(ctx, cErr.BadRequest("Please enter the correct verification code"))
		return
	}

	if params.Type == "email" {
		_, err := h.userM.IsExist(ctx, "email", params.Email, userAuth.Id)
		if err != nil {
			FailByErr(ctx, cErr.BadRequest("The email phone has been bound to another account"))
			return
		}
		h.userM.Update(ctx, user.ID, map[string]any{
			"email": params.Email,
		})
	} else if params.Type == "mobile" {
		_, err := h.userM.IsExist(ctx, "mobile", params.Mobile, userAuth.Id)
		if err != nil {
			FailByErr(ctx, cErr.BadRequest("The mobile phone has been bound to another account"))
			return
		}
		h.userM.Update(ctx, user.ID, map[string]any{
			"mobile": params.Mobile,
		})
	}
	h.authM.DelVerificationToken(params.AccountVerificationToken)
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
	if !h.userM.ValidatePassword(ctx, userAuth.Id, params.OldPassword) {
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
	fmt.Println(ctx.ClientIP())
	userAuth := header.GetUserAuth(ctx)
	result, total, err := h.userScoreLogM.List(ctx, userAuth.Id)
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
	userAuth := header.GetUserAuth(ctx)
	result, total, err := h.userMoneyLogM.List(ctx, userAuth.Id)
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
		FailByErr(ctx, cErr.NotFound("Account not exist"))
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
