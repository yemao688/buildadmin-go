package user

import (
	"bytes"
	"encoding/json"
	adminmodel "buildadmin-go/internal/admin/repository/user"
	"buildadmin-go/internal/admin/validate"
	model "buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"
	passwordutil "buildadmin-go/internal/pkg/password"
	"io"
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type UserHandler struct {
	Base
	log   *zap.Logger
	userM *adminmodel.UserRepository
}

func NewUserHandler(log *zap.Logger, userM *adminmodel.UserRepository) *UserHandler {
	return &UserHandler{Base: NewBase(userM), log: log, userM: userM}
}

func validateAccountStatusValue(value any) error {
	status, ok := value.(string)
	if !ok || (status != "enable" && status != "disable") {
		return cErr.BadRequest("status must be enable or disable")
	}
	return nil
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
	AdminID  int32  `json:"admin_id"`
	Username string `json:"username" binding:"required"`
	Nickname string `json:"nickname" binding:"required"`
	Email    string `json:"email"`
	Mobile   string `json:"mobile"`
	Avatar   string `json:"avatar"`
	JoinIP   string `json:"join_ip"`
	JoinTime int64  `json:"join_time"`
	Password string `json:"password"`
	Status   string `json:"status" binding:"oneof=enable disable"`
}

func (v User) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"username.min":      "username>2 and username<15",
		"username.max":      "username>2 and username<15",
		"password.password": "password invalid",
	}
}

func (h *UserHandler) Add(ctx *gin.Context) {
	bodyBytes, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		FailByErr(ctx, cErr.BadRequest("invalid request body"))
		return
	}
	ctx.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	adminID, hasAdminID, err := requestedAdminID(bodyBytes)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	var params User
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}
	usernameExists, err := h.userM.UsernameExists(ctx, params.Username)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	if usernameExists {
		FailByErr(ctx, cErr.BadRequest("Account exist"))
		return
	}
	if params.Password == "" {
		FailByErr(ctx, cErr.BadRequest("Please input correct password"))
		return
	}

	var user model.User
	copier.Copy(&user, params)
	if hasAdminID {
		user.AdminID = adminID
	}
	user.Password, err = passwordutil.Hash(params.Password)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	err = h.userM.Add(ctx, &user)
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
	bodyBytes, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		FailByErr(ctx, cErr.BadRequest("invalid request body"))
		return
	}
	ctx.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var switchReq struct {
		ID     int32  `json:"id"`
		Status string `json:"status"`
	}
	isSwitch := false
	if err := json.Unmarshal(bodyBytes, &switchReq); err == nil && switchReq.ID != 0 {
		var raw map[string]any
		if err := json.Unmarshal(bodyBytes, &raw); err == nil && len(raw) == 2 {
			_, hasID := raw["id"]
			_, hasStatus := raw["status"]
			isSwitch = hasID && hasStatus
		}
	}
	if isSwitch {
		if err := validateAccountStatusValue(switchReq.Status); err != nil {
			FailByErr(ctx, err)
			return
		}
		if err := h.userM.UpdateStatus(ctx, switchReq.ID, switchReq.Status); err != nil {
			FailByErr(ctx, err)
			return
		}
		Success(ctx, "")
		return
	}

	var params = struct {
		IDS
		User
	}{}
	if err = ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}
	adminID, hasAdminID, err := requestedAdminID(bodyBytes)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	user, err := h.userM.GetOne(ctx, params.ID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	currentAdminID := user.AdminID

	copier.Copy(&user, params)
	if hasAdminID {
		user.AdminID = adminID
	} else {
		user.AdminID = currentAdminID
	}
	err = h.userM.Edit(ctx, &user, params.Password)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func requestedAdminID(body []byte) (int32, bool, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return 0, false, cErr.BadRequest("invalid request body")
	}
	value, present := raw["admin_id"]
	if !present {
		return 0, false, nil
	}
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return 0, true, cErr.BadRequest("admin_id must be a positive administrator id")
	}
	var id int64
	if err := json.Unmarshal(value, &id); err != nil || id <= 0 || id > math.MaxInt32 {
		return 0, true, cErr.BadRequest("admin_id must be a positive administrator id")
	}
	return int32(id), true, nil
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
