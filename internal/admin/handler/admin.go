package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	"buildadmin-go/internal/pkg/requesttx"
	"buildadmin-go/internal/pkg/response"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/admin/service"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/tree"
	"buildadmin-go/internal/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type AdminHandler struct {
	Base
	log    *zap.Logger
	adminM *adminmodel.AdminRepository
	authM  *adminmodel.AuthRepository
	svc    *service.AdminService
}

func NewAdminHandler(log *zap.Logger, adminM *adminmodel.AdminRepository, authM *adminmodel.AuthRepository, svc *service.AdminService) *AdminHandler {
	return &AdminHandler{
		Base:   NewBase(adminM),
		log:    log,
		adminM: adminM,
		authM:  authM,
		svc:    svc,
	}
}

func (h *AdminHandler) Index(ctx *gin.Context) {
	if data, matched, err := h.Select(ctx); matched {
		if err != nil {
			response.FailByErr(ctx, err)
			return
		}
		response.Success(ctx, data)
		return
	}

	result, total, err := h.adminM.List(ctx)
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

// NullableParentID is a presence-aware JSON type for parent_id.
//
// API semantics:
//   - Add: omitted / JSON null / 0  => default to current actor for restricted
//     actors, or root (nil) for explicit Unrestricted actors.
//   - Edit: omitted / JSON null => keep current parent unchanged.
//   - Edit: 0 => move to root, allowed only for Unrestricted actors.
//   - Edit: positive integer => move to that parent, subject to scope.
type NullableParentID struct {
	Value *int32
	IsSet bool
}

func (n *NullableParentID) UnmarshalJSON(data []byte) error {
	n.IsSet = true
	if string(data) == "null" {
		n.Value = nil
		return nil
	}
	var v int32
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	n.Value = &v
	return nil
}

type Admin struct {
	Username string           `json:"username" binding:"required,alphanum,min=2,max=15"`
	Nickname string           `json:"nickname" binding:"required"`
	Avatar   string           `json:"avatar" binding:""`
	Email    string           `json:"email" binding:"omitempty,email"`
	Mobile   string           `json:"mobile" binding:"omitempty,phone"`
	Password string           `json:"password" binding:"omitempty,password"`
	Motto    string           `json:"motto"`
	ParentID NullableParentID `json:"parent_id"`
	Status   string           `json:"status" binding:"oneof=enable disable"`
	GroupArr []string         `json:"group_arr" binding:"required"`
}

func (v Admin) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{
		"username.min":      "username>2 and username<15",
		"username.max":      "username>2 and username<15",
		"email.email":       "email error",
		"mobile.phone":      "mobile error",
		"password.password": "password invalid",
	}
}

func actorFromContext(ctx *gin.Context) (data_scope.Actor, error) {
	actor, ok := data_scope.ActorFromContext(ctx)
	if !ok {
		return data_scope.Actor{}, data_scope.ErrScopedAccessDenied
	}
	return actor, nil
}

func (h *AdminHandler) Add(ctx *gin.Context) {
	var params Admin
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	actor, err := actorFromContext(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	adminAuth := header.GetAdminAuth(ctx)
	if err := h.svc.Add(ctx.Request.Context(), adminParams(params), actor, adminAuth.Id); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
	requesttx.InvalidateAfterMutation(ctx, h.authM.InvalidateAll)
}

func adminParams(params Admin) service.AdminParams {
	return service.AdminParams{
		Username: params.Username,
		Nickname: params.Nickname,
		Avatar:   params.Avatar,
		Email:    params.Email,
		Mobile:   params.Mobile,
		Password: params.Password,
		Motto:    params.Motto,
		Status:   params.Status,
		ParentID: service.ParentSelection{Value: params.ParentID.Value, Set: params.ParentID.IsSet},
		GroupArr: params.GroupArr,
	}
}

func (h *AdminHandler) One(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	result, err := h.adminM.GetOne(ctx, int32(id))
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	response.Success(ctx, map[string]interface{}{
		"row": result,
	})
}

// MaybePartialEdit overrides Base.MaybePartialEdit so that switch-unit-cell
// status updates run through the scoped, atomic service/repository path.
func (h *AdminHandler) MaybePartialEdit(ctx *gin.Context, allowedFields map[string]bool, validators ...PartialEditValidator) bool {
	bodyBytes, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		return false
	}
	ctx.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		return false
	}

	if len(m) != 2 {
		return false
	}
	idVal, hasID := m["id"]
	if !hasID {
		return false
	}

	var fieldName string
	var fieldValue any
	for k, v := range m {
		if k != "id" {
			fieldName = k
			fieldValue = v
			break
		}
	}

	if !allowedFields[fieldName] {
		return false
	}

	id := int32(com.StrTo(fmt.Sprintf("%v", idVal)).MustInt())
	for _, validator := range validators {
		if validator == nil {
			continue
		}
		if err := validator(id, fieldName, fieldValue); err != nil {
			response.FailByErr(ctx, err)
			return true
		}
	}

	if fieldName != "status" {
		return false
	}
	status, ok := fieldValue.(string)
	if !ok {
		response.FailByErr(ctx, cErr.BadRequest("status must be a string"))
		return true
	}
	actor, err := actorFromContext(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return true
	}
	if err := h.svc.SwitchStatus(ctx.Request.Context(), id, status, header.GetAdminAuth(ctx).Id, actor); err != nil {
		response.FailByErr(ctx, err)
		return true
	}
	response.Success(ctx, "")
	return true
}

func (h *AdminHandler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{"status": true}) {
		return
	}

	var params = struct {
		dto.IDS
		Admin
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	actor, err := actorFromContext(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	adminAuth := header.GetAdminAuth(ctx)
	if err := h.svc.Edit(ctx.Request.Context(), params.ID, adminParams(params.Admin), actor, adminAuth.Id); err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
	requesttx.InvalidateAfterMutation(ctx, h.authM.InvalidateAll)
}

func (h *AdminHandler) Del(ctx *gin.Context) {
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
	requesttx.InvalidateAfterMutation(ctx, h.authM.InvalidateAll)
}

type adminTreeLeaf struct {
	id       int
	pid      int
	title    string
	children []*adminTreeLeaf
}

func (l *adminTreeLeaf) GetId() int                       { return l.id }
func (l *adminTreeLeaf) GetPid() int                      { return l.pid }
func (l *adminTreeLeaf) GetTitle() string                 { return l.title }
func (l *adminTreeLeaf) GetChildren() interface{}         { return l.children }
func (l *adminTreeLeaf) SetTitle(title string)            { l.title = title }
func (l *adminTreeLeaf) SetChildren(children interface{}) { l.children = children.([]*adminTreeLeaf) }

// Select serves the frontend getSelectData convention for the admin parent
// selector. It returns a flat list of BuildAdmin-style prefixed options,
// scoped to the current actor and excluding the requested node and its
// descendants via the closure table.
func (h *AdminHandler) Select(ctx *gin.Context) (interface{}, bool, error) {
	if s := ctx.Request.FormValue("select"); s == "" {
		return nil, false, nil
	}

	excludeID := int32(com.StrTo(ctx.Request.FormValue("exclude_id")).MustInt())
	keyword := ctx.Request.FormValue("quickSearch")
	admins, err := h.adminM.SelectTree(ctx, excludeID, keyword)
	if err != nil {
		return nil, true, err
	}

	var options []map[string]any
	if keyword == "" {
		options = buildAdminTreeOptions(admins)
	} else {
		options = buildFlatAdminOptions(admins)
	}

	return map[string]interface{}{
		"options": options,
		"remark":  h.GetRemark(ctx),
	}, true, nil
}

func buildAdminTreeOptions(admins []*model.Admin) []map[string]any {
	leaves := make([]*adminTreeLeaf, 0, len(admins))
	for _, a := range admins {
		pid := 0
		if a.ParentID != nil {
			pid = int(*a.ParentID)
		}
		leaves = append(leaves, &adminTreeLeaf{
			id:    int(a.ID),
			pid:   pid,
			title: a.Nickname + "(ID:" + strconv.Itoa(int(a.ID)) + ")",
		})
	}
	assembled := tree.AssembleChild(leaves)
	formatted := tree.GetTreeArray(assembled, 0, false)
	flat := tree.AssembleTree(formatted)

	options := make([]map[string]any, 0, len(flat))
	for _, l := range flat {
		admin := findAdminByID(admins, int32(l.GetId()))
		username := ""
		if admin != nil {
			username = admin.Username + "(ID:" + strconv.Itoa(int(admin.ID)) + ")"
		}
		options = append(options, map[string]any{
			"id":       l.GetId(),
			"nickname": l.GetTitle(),
			"username": username,
		})
	}
	return options
}

func buildFlatAdminOptions(admins []*model.Admin) []map[string]any {
	options := make([]map[string]any, 0, len(admins))
	for _, a := range admins {
		options = append(options, map[string]any{
			"id":       a.ID,
			"nickname": a.Nickname + "(ID:" + strconv.Itoa(int(a.ID)) + ")",
			"username": a.Username + "(ID:" + strconv.Itoa(int(a.ID)) + ")",
		})
	}
	return options
}

func findAdminByID(admins []*model.Admin, id int32) *model.Admin {
	for _, admin := range admins {
		if admin != nil && admin.ID == id {
			return admin
		}
	}
	return nil
}
