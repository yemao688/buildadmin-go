package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"go-build-admin/internal/admin/model"
	securitymodel "go-build-admin/internal/admin/model/security"
	"go-build-admin/internal/pkg/data_scope"
	"go-build-admin/internal/pkg/requesttx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type sensitiveAuditor struct {
	work *securityWork
	rule securitymodel.SecuritySensitiveData
}

func (a *sensitiveAuditor) run() {
	if !a.loadRule() {
		return
	}
	params, ok := a.readRequestParams()
	if !ok {
		return
	}
	primaryParam := a.rule.PrimaryKey
	if primaryParam == "" {
		primaryParam = "id"
	}
	primaryValue, ok := params[primaryParam]
	if !ok {
		a.work.context.Next()
		return
	}
	db, resolvedTable, policy, beforeRow, ok := a.loadBeforeRow(primaryValue)
	if !ok {
		return
	}
	dataFields, ok := a.loadDataFields(db, resolvedTable)
	if !ok {
		return
	}
	idValue, ok := a.auditIdentity(primaryValue)
	if !ok {
		return
	}

	// Let the business handler perform the write first. The transaction
	// wrapper keeps the before snapshot locked and this same DB reads the
	// actual persisted after values below.
	a.work.context.Next()
	if outcome, ok := requesttx.PeekOutcome(a.work.context.Request.Context()); !ok || outcome.BusinessCode != 1 {
		return
	}
	afterRow := map[string]any{}
	query := db.Table(resolvedTable)
	if policy.Table.OwnerColumn != "" {
		query = a.work.scope(query, resolvedTable, policy.Table.OwnerColumn)
	}
	err := query.Where("`"+a.rule.PrimaryKey+"`=?", primaryValue).Take(&afterRow).Error
	if err != nil {
		a.work.abort(http.StatusInternalServerError, "after-state lookup failed")
		return
	}
	logs := a.changedFieldLogs(dataFields, beforeRow, afterRow, idValue)
	if len(logs) == 0 {
		return
	}
	if err := db.Model(&securitymodel.SecuritySensitiveDataLog{}).Create(&logs).Error; err != nil {
		a.work.security.log.Warn("[ DataSecurity ] Sensitive data recording failed:" + err.Error())
		a.work.abort(http.StatusInternalServerError, "security log write failed")
	}
}

func (a *sensitiveAuditor) loadRule() bool {
	err := a.work.resolveRule(a.work.prefix()+"security_sensitive_data", a.work.route(), &a.rule)
	if err == nil {
		return true
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		a.work.context.Next()
		return false
	}
	a.work.abort(http.StatusInternalServerError, "security rule lookup failed")
	return false
}

func (a *sensitiveAuditor) readRequestParams() (map[string]interface{}, bool) {
	body, err := io.ReadAll(a.work.context.Request.Body)
	if err != nil {
		a.work.abort(http.StatusInternalServerError, "Failed to read request body")
		return nil, false
	}
	a.work.context.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	var params map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&params); err != nil {
		a.work.abort(http.StatusBadRequest, "invalid request body")
		return nil, false
	}
	return params, true
}

func (a *sensitiveAuditor) loadBeforeRow(primaryValue any) (*gorm.DB, string, data_scope.RulePolicy, map[string]any, bool) {
	w := a.work
	db := w.db()
	resolvedTable, policy, ownerColumn, err := resolveSecurityTarget(db, w.prefix(), a.rule.DataTable, "sensitive", a.rule.PrimaryKey, nil)
	if err != nil {
		w.abort(http.StatusInternalServerError, "invalid security rule identifier")
		return nil, "", data_scope.RulePolicy{}, nil, false
	}
	if requesttx.Active(w.context.Request.Context()) {
		err := model.NewAdminHierarchy(w.security.config).LockHierarchy(w.context.Request.Context(), db)
		if err != nil {
			w.security.log.Warn("[ DataSecurity ] Hierarchy lock failed:" + err.Error())
			w.abort(http.StatusInternalServerError, "security lock failed")
			return nil, "", data_scope.RulePolicy{}, nil, false
		}
	}
	row := map[string]any{}
	query := db.Table(resolvedTable).Clauses(clause.Locking{Strength: "UPDATE"})
	if ownerColumn != "" {
		query = w.scope(query, resolvedTable, ownerColumn)
	}
	err = query.
		Where("`"+a.rule.PrimaryKey+"`=?", primaryValue).Take(&row).Error
	if err != nil {
		w.security.log.Warn("[ DataSecurity ] Sensitive data recording failed:" + err.Error())
		w.abort(http.StatusInternalServerError, "target lookup failed")
		return nil, "", data_scope.RulePolicy{}, nil, false
	}
	return db, resolvedTable, policy, row, true
}

func (a *sensitiveAuditor) loadDataFields(db *gorm.DB, resolvedTable string) (map[string]string, bool) {
	dataFields := map[string]string{}
	if err := json.Unmarshal([]byte(a.rule.DataFields), &dataFields); err != nil {
		a.work.security.log.Warn("[ DataSecurity ] Sensitive data recording failed:" + err.Error())
		a.work.abort(http.StatusInternalServerError, "invalid security field rule")
		return nil, false
	}
	for field := range dataFields {
		if err := data_scope.ValidateSecurityField(field); err != nil {
			a.work.abort(http.StatusInternalServerError, "forbidden security field")
			return nil, false
		}
		if err := data_scope.ResolveBusinessColumn(db, resolvedTable, field); err != nil {
			a.work.abort(http.StatusInternalServerError, "invalid security field identifier")
			return nil, false
		}
	}
	return dataFields, true
}

func (a *sensitiveAuditor) auditIdentity(primaryValue any) (int32, bool) {
	idText, idErr := normalizePrimaryKeyValue(primaryValue)
	if idErr != nil {
		a.work.abort(http.StatusInternalServerError, "invalid target primary key")
		return 0, false
	}
	idValue, idErr := strconv.ParseInt(idText, 10, 32)
	if idErr != nil {
		// id_value is the existing int32 audit column. Never truncate an
		// int64 or string key; a future schema migration must add text storage.
		a.work.abort(http.StatusInternalServerError, "string or oversized primary keys require text audit storage")
		return 0, false
	}
	return int32(idValue), true
}

func (a *sensitiveAuditor) changedFieldLogs(dataFields map[string]string, beforeRow, afterRow map[string]any, idValue int32) []securitymodel.SecuritySensitiveDataLog {
	w := a.work
	logs := []securitymodel.SecuritySensitiveDataLog{}
	for field, comment := range dataFields {
		beforeV, oldOK := beforeRow[field]
		afterV, newOK := afterRow[field]
		if oldOK && newOK && normalizeAuditValue(beforeV) != normalizeAuditValue(afterV) {
			logs = append(logs, securitymodel.SecuritySensitiveDataLog{
				AdminID:     w.actor.AdminID,
				SensitiveID: a.rule.ID, DataTable: a.rule.DataTable, PrimaryKey: a.rule.PrimaryKey,
				DataField: field, DataComment: comment, IDValue: idValue,
				Before: normalizeAuditValue(beforeV), After: normalizeAuditValue(afterV),
				IP: w.context.ClientIP(), Useragent: w.context.Request.Header.Get("User-Agent"),
			})
		}
	}
	return logs
}
