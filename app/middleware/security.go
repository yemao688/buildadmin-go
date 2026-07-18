package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/app/pkg/requesttx"
	"go-build-admin/conf"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Security struct {
	config   *conf.Configuration
	log      *zap.Logger
	sqlDB    *gorm.DB
	enforcer data_scope.Enforcer
}

func securityScope(ctx *gin.Context, db *gorm.DB, enforcer data_scope.Enforcer, table, ownerColumn string) *gorm.DB {
	if enforcer == nil {
		tx := db.Session(&gorm.Session{})
		_ = tx.AddError(data_scope.ErrScopedAccessDenied)
		return tx
	}
	return enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: table, Column: ownerColumn})
}

func extractOwnerID(row map[string]any, column string) (int32, error) {
	value, ok := row[column]
	if !ok {
		return 0, fmt.Errorf("target owner column %s missing", column)
	}
	var owner int64
	switch v := value.(type) {
	case int:
		owner = int64(v)
	case int32:
		owner = int64(v)
	case int64:
		owner = v
	case uint:
		owner = int64(v)
	case uint32:
		owner = int64(v)
	case uint64:
		if v > uint64(^uint32(0)) {
			return 0, fmt.Errorf("target owner out of range")
		}
		owner = int64(v)
	case float64:
		owner = int64(v)
	default:
		return 0, fmt.Errorf("target owner column %s has unsupported type", column)
	}
	if owner <= 0 || owner > int64(^uint32(0)>>1) {
		return 0, fmt.Errorf("target owner missing")
	}
	return int32(owner), nil
}

func normalizePrimaryKeyValue(value any) (string, error) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return "", fmt.Errorf("empty primary key")
		}
		return v, nil
	case []byte:
		return normalizePrimaryKeyValue(string(v))
	case json.Number:
		return string(v), nil
	case int:
		return strconv.FormatInt(int64(v), 10), nil
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	case float64:
		if v != float64(int64(v)) {
			return "", fmt.Errorf("non-integral primary key")
		}
		return strconv.FormatInt(int64(v), 10), nil
	default:
		return "", fmt.Errorf("unsupported primary key type %T", value)
	}
}

var errBusinessRollback = errors.New("requesttx: business response requested rollback")
var errMissingOutcome = errors.New("requesttx: protected handler produced no staged outcome")
var errDirectResponse = errors.New("requesttx: direct response bypassed staging")

type transactionResponseWriter struct {
	gin.ResponseWriter
	status  int
	started bool
	body    bytes.Buffer
	header  http.Header
}

// normalizeAuditValue gives database driver values a stable representation;
// in particular []byte must be treated as text rather than formatted as a
// numeric byte slice.
func normalizeAuditValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case []byte:
		return string(v)
	case time.Time:
		return v.UTC().Format(time.RFC3339Nano)
	case json.Number:
		return v.String()
	case float32:
		return strconv.FormatFloat(float64(v), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (w *transactionResponseWriter) WriteHeader(code int) {
	if !w.started && code > 0 {
		w.status = code
	}
}
func (w *transactionResponseWriter) WriteHeaderNow() {
	if !w.started {
		w.started = true
	}
}
func (w *transactionResponseWriter) Header() http.Header { return w.header }
func (w *transactionResponseWriter) Write(p []byte) (int, error) {
	w.WriteHeaderNow()
	return w.body.Write(p)
}
func (w *transactionResponseWriter) WriteString(s string) (int, error) {
	w.WriteHeaderNow()
	return w.body.WriteString(s)
}
func (w *transactionResponseWriter) Status() int   { return w.status }
func (w *transactionResponseWriter) Size() int     { return w.body.Len() }
func (w *transactionResponseWriter) Written() bool { return w.started }
func (w *transactionResponseWriter) flush() {
	for key, values := range w.header {
		w.ResponseWriter.Header()[key] = append([]string(nil), values...)
	}
	if !w.started {
		return
	}
	w.ResponseWriter.WriteHeader(w.status)
	if w.body.Len() > 0 {
		_, _ = w.ResponseWriter.Write(w.body.Bytes())
	}
}

func NewSecurity(
	config *conf.Configuration,
	log *zap.Logger,
	sqlDB *gorm.DB,
	enforcer data_scope.Enforcer,
) *Security {
	return &Security{
		config:   config,
		log:      log,
		sqlDB:    sqlDB,
		enforcer: enforcer,
	}
}

type AtomicRoute struct {
	Route  string
	Action string
	Method string
}

var atomicRoutes = map[AtomicRoute]struct{}{
	{Route: "auth/admin", Action: "add", Method: http.MethodPost}:                     {},
	{Route: "auth/admin", Action: "edit", Method: http.MethodPost}:                    {},
	{Route: "auth/admin", Action: "del", Method: http.MethodDelete}:                   {},
	{Route: "auth/group", Action: "add", Method: http.MethodPost}:                     {},
	{Route: "auth/group", Action: "edit", Method: http.MethodPost}:                    {},
	{Route: "auth/group", Action: "del", Method: http.MethodDelete}:                   {},
	{Route: "auth/rule", Action: "add", Method: http.MethodPost}:                      {},
	{Route: "auth/rule", Action: "edit", Method: http.MethodPost}:                     {},
	{Route: "auth/rule", Action: "del", Method: http.MethodDelete}:                    {},
	{Route: "routine/config", Action: "add", Method: http.MethodPost}:                 {},
	{Route: "routine/config", Action: "edit", Method: http.MethodPost}:                {},
	{Route: "routine/config", Action: "del", Method: http.MethodDelete}:               {},
	{Route: "user/user", Action: "add", Method: http.MethodPost}:                      {},
	{Route: "user/user", Action: "edit", Method: http.MethodPost}:                     {},
	{Route: "user/user", Action: "del", Method: http.MethodDelete}:                    {},
	{Route: "security/datarecycle", Action: "add", Method: http.MethodPost}:           {},
	{Route: "security/datarecycle", Action: "edit", Method: http.MethodPost}:          {},
	{Route: "security/datarecycle", Action: "del", Method: http.MethodDelete}:         {},
	{Route: "security/datarecyclelog", Action: "restore", Method: http.MethodPost}:    {},
	{Route: "security/datarecyclelog", Action: "del", Method: http.MethodDelete}:      {},
	{Route: "security/sensitivedata", Action: "add", Method: http.MethodPost}:         {},
	{Route: "security/sensitivedata", Action: "edit", Method: http.MethodPost}:        {},
	{Route: "security/sensitivedata", Action: "del", Method: http.MethodDelete}:       {},
	{Route: "security/sensitivedatalog", Action: "rollback", Method: http.MethodPost}: {},
	{Route: "security/sensitivedatalog", Action: "del", Method: http.MethodDelete}:    {},
}

// atomicRoutesMu guards atomicRoutes: CRUD generation registers capabilities
// at request time while normal traffic reads them concurrently.
var atomicRoutesMu sync.RWMutex

// normalizeAtomicRoute lowercases route/action so generated CamelCase
// controllers (e.g. userOrder) match the lowercased lookup path.
func normalizeAtomicRoute(route AtomicRoute) AtomicRoute {
	return AtomicRoute{
		Route:  strings.ToLower(route.Route),
		Action: strings.ToLower(route.Action),
		Method: route.Method,
	}
}

// RegisterAtomicRoute lets router construction be the source of truth for
// capability registration. Seed entries remain for deployments that construct
// Security in isolation (and for compatibility tests).
func RegisterAtomicRoute(route AtomicRoute) {
	atomicRoutesMu.Lock()
	defer atomicRoutesMu.Unlock()
	atomicRoutes[normalizeAtomicRoute(route)] = struct{}{}
}

// UnregisterAtomicRoute removes a runtime-registered capability. Used when a
// generation fails after routes were registered, and when a CRUD module is
// deleted in the current process.
func UnregisterAtomicRoute(route AtomicRoute) {
	atomicRoutesMu.Lock()
	defer atomicRoutesMu.Unlock()
	delete(atomicRoutes, normalizeAtomicRoute(route))
}

func normalizeRouteAction(fullPath string) (string, string, bool) {
	parts := strings.Split(strings.Trim(fullPath, "/"), "/")
	if len(parts) != 3 || parts[0] != "admin" {
		return "", "", false
	}
	controller := strings.ReplaceAll(parts[1], ".", "/")
	return strings.ToLower(controller), strings.ToLower(parts[2]), true
}

func AtomicRouteCapability(c *gin.Context) (AtomicRoute, bool) {
	route, action, ok := normalizeRouteAction(c.FullPath())
	if !ok {
		return AtomicRoute{}, false
	}
	cap := AtomicRoute{Route: route, Action: action, Method: c.Request.Method}
	atomicRoutesMu.RLock()
	_, ok = atomicRoutes[cap]
	atomicRoutesMu.RUnlock()
	return cap, ok
}

func (m *Security) hasSecurityRule(c *gin.Context, route string) (bool, error) {
	if m.config == nil || m.sqlDB == nil {
		return false, errors.New("security rule database is unavailable")
	}
	if route == "" {
		return false, nil
	}
	logical := "security_sensitive_data"
	if c.Request.Method == http.MethodDelete {
		logical = "security_data_recycle"
	}
	var count int64
	err := m.sqlDB.Table(m.config.Database.Prefix+logical).
		Where("status = ? AND controller_as = ?", "1", route).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Handler opens the request transaction before the protected handler runs.
// Response bodies are staged by the response helpers and emitted only after
// GORM commits successfully.
func (m *Security) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost && c.Request.Method != http.MethodDelete {
			m.workHandler()(c)
			return
		}
		if _, ok := AtomicRouteCapability(c); !ok {
			route, _, _ := normalizeRouteAction(c.FullPath())
			hasRule, err := m.hasSecurityRule(c, route)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "security rule lookup failed"})
				return
			}
			if hasRule {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "atomic route capability missing"})
				return
			}
			m.workHandler()(c)
			return
		}
		var outcome requesttx.Outcome
		var hasOutcome bool
		var txErr error
		originalWriter := c.Writer
		bufferedWriter := &transactionResponseWriter{ResponseWriter: originalWriter, status: http.StatusOK, header: make(http.Header)}
		c.Writer = bufferedWriter
		finishRequest := func() {
			if c.Request == nil {
				return
			}
			boundContext := c.Request.Context()
			requesttx.Finish(boundContext)
			c.Request = c.Request.WithContext(requesttx.Unbind(boundContext))
		}
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					requesttx.DiscardOutcome(c.Request.Context())
					finishRequest()
					c.Writer = originalWriter
					panic(recovered)
				}
			}()
			txErr = m.sqlDB.Transaction(func(tx *gorm.DB) error {
				bound := requesttx.Bind(c.Request.Context(), tx)
				c.Request = c.Request.WithContext(bound)
				m.workHandler()(c)
				outcome, hasOutcome = requesttx.PeekOutcome(c.Request.Context())
				if !hasOutcome {
					return errMissingOutcome
				}
				if bufferedWriter.Written() {
					return errDirectResponse
				}
				if outcome.BusinessCode != 1 {
					return errBusinessRollback
				}
				if c.IsAborted() || c.Writer.Status() >= http.StatusBadRequest {
					return errors.New("requesttx: request aborted")
				}
				return nil
			})
		}()
		if txErr != nil {
			if errors.Is(txErr, errBusinessRollback) {
				if out, ok := requesttx.TakeOutcome(c.Request.Context()); ok {
					c.Writer = originalWriter
					c.JSON(out.HTTPCode, gin.H{"code": out.BusinessCode, "data": out.Data, "msg": out.Message, "time": 0})
				}
				finishRequest()
				return
			}
			requesttx.DiscardOutcome(c.Request.Context())
			finishRequest()
			if bufferedWriter.Written() {
				m.log.Warn("[ DataSecurity ] direct response discarded after transaction failure")
			}
			c.Writer = originalWriter
			c.JSON(http.StatusInternalServerError, gin.H{"code": 0, "data": nil, "msg": "transaction failed", "time": 0})
			return
		}
		c.Writer = originalWriter
		bufferedWriter.flush()
		if out, ok := requesttx.TakeOutcome(c.Request.Context()); ok {
			c.JSON(out.HTTPCode, gin.H{"code": out.BusinessCode, "data": out.Data, "msg": out.Message, "time": 0})
		}
		finishRequest()
	}
}

func (m *Security) workHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := data_scope.ActorFromContext(c)
		if !ok || data_scope.ValidateActor(actor) != nil {
			m.log.Warn("[ DataSecurity ] missing or invalid actor; abort")
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "invalid authenticated actor"})
			return
		}
		abort := func(httpCode int, message string) {
			ctx := c.Request.Context()
			if requesttx.Active(ctx) && requesttx.Stage(ctx, requesttx.Outcome{HTTPCode: httpCode, BusinessCode: 0, Message: message}) {
				c.Abort()
				return
			}
			c.AbortWithStatusJSON(httpCode, gin.H{"error": message})
		}
		scope := func(db *gorm.DB, table, ownerColumn string) *gorm.DB {
			return securityScope(c, db, m.enforcer, table, ownerColumn)
		}
		getPath := func(c *gin.Context) string {
			route, _, ok := normalizeRouteAction(c.FullPath())
			if !ok {
				return ""
			}
			return route
		}
		executionRule := func(table, route string, rule any) error {
			if err := data_scope.ValidateTablePrefix(m.config.Database.Prefix); err != nil {
				return err
			}
			if err := data_scope.ValidateIdentifier(table); err != nil {
				return err
			}
			db := requesttx.DB(c.Request.Context())
			if db == nil {
				db = m.sqlDB
			}
			adminTable := m.config.Database.Prefix + "admin"
			base := db.Table(table).
				Joins("JOIN `"+adminTable+"` AS rule_owner ON rule_owner.id = `"+table+"`.admin_id").
				Where("`"+table+"`.status = ? AND `"+table+"`.controller_as = ?", "1", route)
			if !actor.Unrestricted {
				closure := m.config.Database.Prefix + "admin_closure"
				base = base.Joins("JOIN `"+closure+"` AS owner_scope ON owner_scope.ancestor_id = `"+table+"`.admin_id AND owner_scope.descendant_id = ?", actor.AdminID).
					Order("owner_scope.depth ASC").Order("`" + table + "`.admin_id ASC")
			} else {
				// Unrestricted is deterministic too: only a rule owned by the
				// hierarchy root is eligible, and the join proves that owner exists.
				base = base.Where("rule_owner.parent_id IS NULL").Order("rule_owner.id ASC")
			}
			return base.First(rule).Error
		}

		//记录删除数据操作
		if c.Request.Method == http.MethodDelete {
			//是否配置回收规则
			recycle := model.SecurityDataRecycle{}
			err := executionRule(m.config.Database.Prefix+"security_data_recycle", getPath(c), &recycle)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					c.Next()
					return
				}
				abort(http.StatusInternalServerError, "security rule lookup failed")
				return
			}

			var params = struct {
				Ids []string `form:"ids[]" binding:"required"`
			}{}
			if err := c.ShouldBindQuery(&params); err != nil || len(params.Ids) == 0 {
				abort(http.StatusBadRequest, "invalid ids")
				return
			}
			seenIDs := make(map[string]struct{}, len(params.Ids))
			normalizedIDs := make([]string, 0, len(params.Ids))
			for _, id := range params.Ids {
				if id == "" {
					abort(http.StatusBadRequest, "ids must be positive")
					return
				}
				if _, exists := seenIDs[id]; !exists {
					seenIDs[id] = struct{}{}
					normalizedIDs = append(normalizedIDs, id)
				}
			}
			sort.Slice(normalizedIDs, func(i, j int) bool { return normalizedIDs[i] < normalizedIDs[j] })

			rows := []map[string]any{}
			db := requesttx.DB(c.Request.Context())
			if db == nil {
				db = m.sqlDB
			}
			resolvedTable, err := data_scope.ResolveBusinessTable(db, m.config.Database.Prefix, recycle.DataTable)
			policy, policyErr := data_scope.ResolveRulePolicy(db, m.config.Database.Prefix, recycle.DataTable, "recycle", recycle.PrimaryKey, nil, recycle.OwnerColumn)
			if err != nil || policyErr != nil || data_scope.ResolveBusinessColumn(db, resolvedTable, recycle.PrimaryKey) != nil {
				abort(http.StatusInternalServerError, "invalid security rule identifier")
				return
			}
			err = scope(db.Table(resolvedTable), resolvedTable, policy.Table.OwnerColumn).Clauses(clause.Locking{Strength: "UPDATE"}).Where("`"+recycle.PrimaryKey+"` IN ?", normalizedIDs).Find(&rows).Error
			if err != nil {
				m.log.Warn("[ DataSecurity ] Failed to recycle data:" + err.Error())
				abort(http.StatusInternalServerError, "target lookup failed")
				return
			}
			if len(rows) != len(normalizedIDs) {
				abort(http.StatusForbidden, "target scope incomplete")
				return
			}
			matched := make(map[string]struct{}, len(rows))
			for _, row := range rows {
				id, err := normalizePrimaryKeyValue(row[recycle.PrimaryKey])
				if err != nil {
					abort(http.StatusInternalServerError, "invalid target primary key")
					return
				}
				matched[id] = struct{}{}
			}
			for _, id := range normalizedIDs {
				if _, ok := matched[id]; !ok {
					abort(http.StatusForbidden, "target scope incomplete")
					return
				}
			}

			//创建删除记录
			recycleLogs := []model.SecurityDataRecycleLog{}
			for _, v := range rows {
				data, err := json.Marshal(v)
				if err != nil {
					abort(http.StatusInternalServerError, "snapshot failed")
					return
				}
				targetOwner, ownerErr := extractOwnerID(v, policy.Table.OwnerColumn)
				if ownerErr != nil {
					abort(http.StatusInternalServerError, "target owner missing")
					return
				}
				recycleLogs = append(recycleLogs, model.SecurityDataRecycleLog{
					AdminID:       actor.AdminID,
					TargetAdminID: targetOwner,
					IsCommitted:   1,
					RecycleID:     recycle.ID,
					Data:          string(data),
					DataTable:     recycle.DataTable,
					PrimaryKey:    recycle.PrimaryKey,
					IP:            c.ClientIP(),
					Useragent:     c.Request.Header.Get("User-Agent"),
				})
			}
			if len(recycleLogs) == 0 {
				abort(http.StatusInternalServerError, "empty target set")
				return
			}

			err = db.Model(&model.SecurityDataRecycleLog{}).Create(&recycleLogs).Error
			if err != nil {
				m.log.Warn("[ DataSecurity ] Failed to recycle data:" + err.Error())
				abort(http.StatusInternalServerError, "security log write failed")
				return
			}
			c.Next()
			outcome, hasOutcome := requesttx.PeekOutcome(c.Request.Context())
			if !hasOutcome || outcome.BusinessCode != 1 {
				return
			}
			var remaining int64
			if err := db.Table(resolvedTable).Where("`"+recycle.PrimaryKey+"` IN ?", normalizedIDs).Count(&remaining).Error; err != nil {
				abort(http.StatusInternalServerError, "delete verification failed")
				return
			}
			if remaining != 0 {
				abort(http.StatusInternalServerError, "delete was not complete")
				return
			}
			return
		}

		// 记录修改数据操作
		if c.Request.Method == http.MethodPost {
			//是否配置敏感数据规则
			sensitive := model.SecuritySensitiveData{}
			err := executionRule(m.config.Database.Prefix+"security_sensitive_data", getPath(c), &sensitive)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					c.Next()
					return
				}
				abort(http.StatusInternalServerError, "security rule lookup failed")
				return
			}

			//读取请求参数
			body, err := io.ReadAll(c.Request.Body)
			if err != nil {
				abort(http.StatusInternalServerError, "Failed to read request body")
				return
			}

			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
			var params map[string]interface{}
			decoder := json.NewDecoder(bytes.NewReader(body))
			decoder.UseNumber()
			if err := decoder.Decode(&params); err != nil {
				abort(http.StatusBadRequest, "invalid request body")
				return
			}
			primaryParam := sensitive.PrimaryKey
			if primaryParam == "" {
				primaryParam = "id"
			}
			primaryValue, ok := params[primaryParam]
			if !ok {
				c.Next()
				return
			}

			//查询需要修改的记录
			db := requesttx.DB(c.Request.Context())
			if db == nil {
				db = m.sqlDB
			}
			resolvedTable, err := data_scope.ResolveBusinessTable(db, m.config.Database.Prefix, sensitive.DataTable)
			policy, policyErr := data_scope.ResolveRulePolicy(db, m.config.Database.Prefix, sensitive.DataTable, "sensitive", sensitive.PrimaryKey, nil, sensitive.OwnerColumn)
			if err != nil || policyErr != nil || data_scope.ResolveBusinessColumn(db, resolvedTable, sensitive.PrimaryKey) != nil {
				abort(http.StatusInternalServerError, "invalid security rule identifier")
				return
			}
			row := map[string]any{}
			err = scope(db.Table(resolvedTable), resolvedTable, policy.Table.OwnerColumn).Clauses(clause.Locking{Strength: "UPDATE"}).Where("`"+sensitive.PrimaryKey+"`=?", primaryValue).Take(&row).Error
			if err != nil {
				m.log.Warn("[ DataSecurity ] Sensitive data recording failed:" + err.Error())
				abort(http.StatusInternalServerError, "target lookup failed")
				return
			}

			//敏感字段
			dataFields := map[string]string{}
			err = json.Unmarshal([]byte(sensitive.DataFields), &dataFields)
			if err != nil {
				m.log.Warn("[ DataSecurity ] Sensitive data recording failed:" + err.Error())
				abort(http.StatusInternalServerError, "invalid security field rule")
				return
			}
			for field := range dataFields {
				if err := data_scope.ValidateSecurityField(field); err != nil {
					abort(http.StatusInternalServerError, "forbidden security field")
					return
				}
				if err := data_scope.ResolveBusinessColumn(db, resolvedTable, field); err != nil {
					abort(http.StatusInternalServerError, "invalid security field identifier")
					return
				}
			}
			targetOwner, ownerErr := extractOwnerID(row, policy.Table.OwnerColumn)
			if ownerErr != nil {
				abort(http.StatusInternalServerError, "target owner missing")
				return
			}
			idText, idErr := normalizePrimaryKeyValue(primaryValue)
			if idErr != nil {
				abort(http.StatusInternalServerError, "invalid target primary key")
				return
			}
			idValue, idErr := strconv.ParseInt(idText, 10, 32)
			if idErr != nil {
				// id_value is the existing int32 audit column. Never truncate an
				// int64 or string key; a future schema migration must add text storage.
				abort(http.StatusInternalServerError, "string or oversized primary keys require text audit storage")
				return
			}

			// Let the business handler perform the write first. The transaction
			// wrapper keeps the before snapshot locked and this same DB reads the
			// actual persisted after values below.
			c.Next()
			if outcome, ok := requesttx.PeekOutcome(c.Request.Context()); !ok || outcome.BusinessCode != 1 {
				return
			}
			afterRow := map[string]any{}
			err = scope(db.Table(resolvedTable), resolvedTable, policy.Table.OwnerColumn).Where("`"+sensitive.PrimaryKey+"`=?", primaryValue).Take(&afterRow).Error
			if err != nil {
				abort(http.StatusInternalServerError, "after-state lookup failed")
				return
			}
			sensitiveDataLogs := []model.SecuritySensitiveDataLog{}
			for k, v := range dataFields {
				beforeV, oldOk := row[k]
				afterV, newOk := afterRow[k]
				if oldOk && newOk && normalizeAuditValue(beforeV) != normalizeAuditValue(afterV) {
					sensitiveDataLogs = append(sensitiveDataLogs, model.SecuritySensitiveDataLog{
						AdminID: actor.AdminID, TargetAdminID: targetOwner, IsCommitted: 1,
						SensitiveID: sensitive.ID, DataTable: sensitive.DataTable, PrimaryKey: sensitive.PrimaryKey,
						DataField: k, DataComment: v, IDValue: int32(idValue),
						Before: normalizeAuditValue(beforeV), After: normalizeAuditValue(afterV),
						IP: c.ClientIP(), Useragent: c.Request.Header.Get("User-Agent"),
					})
				}
			}
			if len(sensitiveDataLogs) == 0 {
				return
			}
			err = db.Model(&model.SecuritySensitiveDataLog{}).Create(&sensitiveDataLogs).Error
			if err != nil {
				m.log.Warn("[ DataSecurity ] Sensitive data recording failed:" + err.Error())
				abort(http.StatusInternalServerError, "security log write failed")
				return
			}
			return
		}
		c.Next()
	}
}
