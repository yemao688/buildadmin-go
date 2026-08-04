package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/admin/service"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/validator"
	"bytes"
	"encoding/json"
	"io"
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type UserHandler struct {
	Base
	log   *zap.Logger
	userM *adminmodel.UserRepository
	svc   *service.UserService
}

func NewUserHandler(log *zap.Logger, userM *adminmodel.UserRepository, svc *service.UserService) *UserHandler {
	return &UserHandler{Base: NewBase(userM), log: log, userM: userM, svc: svc}
}

func (h *UserHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		response.Success(ctx, data)
		return
	}

	result, total, err := h.userM.List(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, map[string]interface{}{
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

func (v User) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{
		"username.min":      "username>2 and username<15",
		"username.max":      "username>2 and username<15",
		"password.password": "password invalid",
	}
}

func (h *UserHandler) Add(ctx *gin.Context) {
	bodyBytes, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		response.FailByErr(ctx, cErr.BadRequest("invalid request body"))
		return
	}
	ctx.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	adminID, hasAdminID, err := requestedAdminID(bodyBytes)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	var params User
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	actor, err := actorFromContext(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	userParams := userParams(params)
	if hasAdminID {
		userParams.AdminID = &adminID
	}
	if err := h.svc.Add(ctx.Request.Context(), userParams, actor); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}

func userParams(params User) service.UserParams {
	return service.UserParams{
		Username: params.Username,
		Nickname: params.Nickname,
		Email:    params.Email,
		Mobile:   params.Mobile,
		Avatar:   params.Avatar,
		JoinIP:   params.JoinIP,
		JoinTime: params.JoinTime,
		Password: params.Password,
		Status:   params.Status,
	}
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
		response.FailByErr(ctx, err)
		return
	}

	result, err := h.userM.DealData(ctx, &user)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	if userId != "" {
		response.Success(ctx, map[string]interface{}{
			"user": result,
		})
		return
	}
	response.Success(ctx, map[string]interface{}{
		"row": result,
	})
}

func (h *UserHandler) Edit(ctx *gin.Context) {
	bodyBytes, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		response.FailByErr(ctx, cErr.BadRequest("invalid request body"))
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
		actor, err := actorFromContext(ctx)
		if err != nil {
			response.FailByErr(ctx, err)
			return
		}
		if err := h.svc.UpdateStatus(ctx.Request.Context(), switchReq.ID, switchReq.Status, actor); err != nil {
			response.FailByErr(ctx, err)
			return
		}
		response.Success(ctx, "")
		return
	}

	var params = struct {
		dto.IDS
		User
	}{}
	if err = ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	adminID, hasAdminID, err := requestedAdminID(bodyBytes)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	actor, err := actorFromContext(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	userParams := userParams(params.User)
	if hasAdminID {
		userParams.AdminID = &adminID
	}
	if err := h.svc.Edit(ctx.Request.Context(), params.ID, userParams, actor); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
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
	var params validator.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	actor, err := actorFromContext(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	err = h.svc.Del(ctx.Request.Context(), params.Ids, actor)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}

func (h *UserHandler) Select(ctx *gin.Context) (interface{}, bool) {
	if s := ctx.Request.FormValue("select"); s == "" {
		return nil, false
	}

	result, total, err := h.userM.List(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return nil, false
	}

	list := []map[string]any{}
	for _, v := range result {
		list = append(list, map[string]any{
			"id":            v.ID,
			"username_text": v.Username + "(ID:+" + strconv.Itoa(int(v.ID)) + ")",
		})
	}

	return map[string]any{
		"list":   list,
		"total":  total,
		"remark": h.GetRemark(ctx),
	}, true
}
