package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"

	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/pkg/data_scope"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/header"
	"go-build-admin/app/pkg/random"
	"go-build-admin/app/pkg/tree"
	"go-build-admin/utils"

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
	if data, matched, err := h.Select(ctx); matched {
		if err != nil {
			FailByErr(ctx, err)
			return
		}
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

func (v Admin) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
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

// resolveParentIDForAdd implements the Add semantics: omitted/null/0 means
// default to the current actor for restricted actors, or root for Unrestricted.
func resolveParentIDForAdd(p NullableParentID, actor data_scope.Actor) (*int32, error) {
	if !p.IsSet || p.Value == nil || *p.Value == 0 {
		if actor.Unrestricted {
			return nil, nil
		}
		return &actor.AdminID, nil
	}
	if *p.Value < 0 {
		return nil, cErr.BadRequest("parent_id must be non-negative")
	}
	return p.Value, nil
}

// resolveParentIDForEdit implements the Edit semantics: omitted/null keeps the
// current parent; 0 moves to root only for Unrestricted actors; positive values
// move to that parent. The returned bool indicates whether the parent changed.
func resolveParentIDForEdit(p NullableParentID, current *int32, actor data_scope.Actor) (*int32, bool, error) {
	if !p.IsSet || p.Value == nil {
		return current, false, nil
	}
	if *p.Value == 0 {
		if !actor.Unrestricted {
			return nil, false, cErr.BadRequest("restricted actor cannot move administrator to root")
		}
		return nil, !int32PtrEqual(current, nil), nil
	}
	if *p.Value < 0 {
		return nil, false, cErr.BadRequest("parent_id must be non-negative")
	}
	return p.Value, !int32PtrEqual(current, p.Value), nil
}

// int32PtrEqual reports whether two optional int32 pointers refer to the same
// value (or both nil).
func int32PtrEqual(a, b *int32) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// isMovingUnderSelf reports whether an existing node would be moved under itself.
// For a new administrator (nodeID == 0) this is always false because the node
// does not exist yet and the actor may legitimately create a direct subordinate.
func isMovingUnderSelf(nodeID int32, parentID *int32) bool {
	return nodeID > 0 && parentID != nil && *parentID == nodeID
}

func setAdminPassword(admin *model.Admin, plaintext string) {
	admin.Salt = random.Build("alnum", 16)
	admin.Password = utils.EncryptPassword(plaintext, admin.Salt)
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

	actor, err := actorFromContext(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	adminAuth := header.GetAdminAuth(ctx)
	if len(params.GroupArr) > 0 {
		if err := h.CheckGroupAuth(ctx, params.GroupArr, adminAuth.Id); err != nil {
			FailByErr(ctx, err)
			return
		}
	}

	parentID, err := resolveParentIDForAdd(params.ParentID, actor)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	if parentID != nil {
		if err := h.adminM.CheckParentInScope(ctx, *parentID); err != nil {
			FailByErr(ctx, err)
			return
		}
	}

	var admin model.Admin
	copier.Copy(&admin, params)

	setAdminPassword(&admin, params.Password)
	admin.ParentID = parentID

	if err := h.adminM.Add(ctx, admin, params.GroupArr); err != nil {
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

	Success(ctx, map[string]interface{}{
		"row": result,
	})
}

// MaybePartialEdit overrides Base.MaybePartialEdit so that switch-unit-cell
// status updates run through the scoped, atomic AdminModel.SwitchStatus.
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
			FailByErr(ctx, err)
			return true
		}
	}

	if fieldName != "status" {
		return false
	}
	status, ok := fieldValue.(string)
	if !ok {
		FailByErr(ctx, cErr.BadRequest("status must be a string"))
		return true
	}
	if err := validateAccountStatusValue(status); err != nil {
		FailByErr(ctx, err)
		return true
	}
	if err := h.adminM.SwitchStatus(ctx, id, status); err != nil {
		FailByErr(ctx, err)
		return true
	}
	Success(ctx, "")
	return true
}

func (h *AdminHandler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{"status": true}, func(id int32, fieldName string, fieldValue any) error {
		if fieldName != "status" {
			return nil
		}
		if err := validateAccountStatusValue(fieldValue); err != nil {
			return err
		}
		status := fieldValue.(string)
		adminAuth := header.GetAdminAuth(ctx)
		if adminAuth.Id == id && status == "disable" {
			return cErr.BadRequest("Please use another administrator account to disable the current account!")
		}
		return nil
	}) {
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

	actor, err := actorFromContext(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	adminAuth := header.GetAdminAuth(ctx)
	if adminAuth.Id == admin.ID && params.Status == "disable" {
		FailByErr(ctx, cErr.BadRequest("Please use another administrator account to disable the current account!"))
		return
	}

	parentID, changed, err := resolveParentIDForEdit(params.ParentID, admin.ParentID, actor)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	if isMovingUnderSelf(admin.ID, parentID) {
		FailByErr(ctx, cErr.BadRequest("cannot move an administrator under itself"))
		return
	}

	omit := []string{"login_failure", "last_login_time", "parent_id"}
	if params.Password == "" {
		omit = append(omit, "password", "salt")
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

	if changed && parentID != nil {
		if err := h.adminM.CheckParentInScope(ctx, *parentID); err != nil {
			FailByErr(ctx, err)
			return
		}
	}

	if err := copier.Copy(&admin, params); err != nil {
		FailByErr(ctx, err)
		return
	}
	// Hash only after copier.Copy: the DTO password is plaintext and must never
	// survive into the model passed to the transactional writer.
	if params.Password != "" {
		setAdminPassword(&admin, params.Password)
	}
	admin.ParentID = parentID

	err = h.adminM.Edit(ctx, admin, changed, parentID, omit, params.GroupArr)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func validateAccountStatusValue(value any) error {
	status, ok := value.(string)
	if !ok || (status != "enable" && status != "disable") {
		return cErr.BadRequest("status must be enable or disable")
	}
	return nil
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
		options = append(options, map[string]any{
			"id":       l.GetId(),
			"nickname": l.GetTitle(),
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
		})
	}
	return options
}
