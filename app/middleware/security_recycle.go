package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"go-build-admin/app/admin/model"
	securitymodel "go-build-admin/app/admin/model/security"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/app/pkg/requesttx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type recycleAuditor struct {
	work *securityWork
	rule securitymodel.SecurityDataRecycle
}

func (a *recycleAuditor) run() {
	if !a.loadRule() {
		return
	}
	normalizedIDs, ok := a.readIDs()
	if !ok {
		return
	}
	db, resolvedTable, policy, rows, ok := a.loadRows(normalizedIDs)
	if !ok {
		return
	}
	logs, ok := a.snapshotRows(policy, rows)
	if !ok {
		return
	}
	if err := db.Model(&securitymodel.SecurityDataRecycleLog{}).Create(&logs).Error; err != nil {
		a.work.security.log.Warn("[ DataSecurity ] Failed to recycle data:" + err.Error())
		a.work.abort(http.StatusInternalServerError, "security log write failed")
		return
	}

	a.work.context.Next()
	outcome, hasOutcome := requesttx.PeekOutcome(a.work.context.Request.Context())
	if !hasOutcome || outcome.BusinessCode != 1 {
		return
	}
	a.verifyDeletion(db, resolvedTable, normalizedIDs)
}

func (a *recycleAuditor) loadRule() bool {
	err := a.work.resolveRule(a.work.prefix()+"security_data_recycle", a.work.route(), &a.rule)
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

func (a *recycleAuditor) readIDs() ([]string, bool) {
	params := struct {
		Ids []string `form:"ids[]" binding:"required"`
	}{}
	if err := a.work.context.ShouldBindQuery(&params); err != nil || len(params.Ids) == 0 {
		a.work.abort(http.StatusBadRequest, "invalid ids")
		return nil, false
	}
	seenIDs := make(map[string]struct{}, len(params.Ids))
	normalizedIDs := make([]string, 0, len(params.Ids))
	for _, id := range params.Ids {
		if id == "" {
			a.work.abort(http.StatusBadRequest, "ids must be positive")
			return nil, false
		}
		if _, exists := seenIDs[id]; !exists {
			seenIDs[id] = struct{}{}
			normalizedIDs = append(normalizedIDs, id)
		}
	}
	sort.Slice(normalizedIDs, func(i, j int) bool { return normalizedIDs[i] < normalizedIDs[j] })
	return normalizedIDs, true
}

func (a *recycleAuditor) loadRows(normalizedIDs []string) (*gorm.DB, string, data_scope.RulePolicy, []map[string]any, bool) {
	w := a.work
	db := w.db()
	resolvedTable, err := data_scope.ResolveBusinessTable(db, w.prefix(), a.rule.DataTable)
	policy, policyErr := data_scope.ResolveRulePolicy(db, w.prefix(), a.rule.DataTable, "recycle", a.rule.PrimaryKey, nil, a.rule.OwnerColumn)
	if err != nil || policyErr != nil || data_scope.ResolveBusinessColumn(db, resolvedTable, a.rule.PrimaryKey) != nil {
		w.abort(http.StatusInternalServerError, "invalid security rule identifier")
		return nil, "", data_scope.RulePolicy{}, nil, false
	}
	if requesttx.Active(w.context.Request.Context()) {
		if err := model.NewAdminHierarchy(w.security.config).LockHierarchy(w.context.Request.Context(), db); err != nil {
			w.security.log.Warn("[ DataSecurity ] Hierarchy lock failed:" + err.Error())
			w.abort(http.StatusInternalServerError, "security lock failed")
			return nil, "", data_scope.RulePolicy{}, nil, false
		}
	}
	rows := []map[string]any{}
	err = w.scope(db.Table(resolvedTable), resolvedTable, policy.Table.OwnerColumn).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("`"+a.rule.PrimaryKey+"` IN ?", normalizedIDs).Find(&rows).Error
	if err != nil {
		w.security.log.Warn("[ DataSecurity ] Failed to recycle data:" + err.Error())
		w.abort(http.StatusInternalServerError, "target lookup failed")
		return nil, "", data_scope.RulePolicy{}, nil, false
	}
	if len(rows) != len(normalizedIDs) {
		w.abort(http.StatusForbidden, "target scope incomplete")
		return nil, "", data_scope.RulePolicy{}, nil, false
	}
	matched := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		id, err := normalizePrimaryKeyValue(row[a.rule.PrimaryKey])
		if err != nil {
			w.abort(http.StatusInternalServerError, "invalid target primary key")
			return nil, "", data_scope.RulePolicy{}, nil, false
		}
		matched[id] = struct{}{}
	}
	for _, id := range normalizedIDs {
		if _, ok := matched[id]; !ok {
			w.abort(http.StatusForbidden, "target scope incomplete")
			return nil, "", data_scope.RulePolicy{}, nil, false
		}
	}
	return db, resolvedTable, policy, rows, true
}

func (a *recycleAuditor) snapshotRows(policy data_scope.RulePolicy, rows []map[string]any) ([]securitymodel.SecurityDataRecycleLog, bool) {
	w := a.work
	logs := []securitymodel.SecurityDataRecycleLog{}
	for _, row := range rows {
		data, err := json.Marshal(row)
		if err != nil {
			w.abort(http.StatusInternalServerError, "snapshot failed")
			return nil, false
		}
		targetOwner, ownerErr := extractOwnerID(row, policy.Table.OwnerColumn)
		if ownerErr != nil {
			w.abort(http.StatusInternalServerError, "target owner missing")
			return nil, false
		}
		logs = append(logs, securitymodel.SecurityDataRecycleLog{
			AdminID:       w.actor.AdminID,
			TargetAdminID: targetOwner,
			IsCommitted:   1,
			RecycleID:     a.rule.ID,
			Data:          string(data),
			DataTable:     a.rule.DataTable,
			PrimaryKey:    a.rule.PrimaryKey,
			IP:            w.context.ClientIP(),
			Useragent:     w.context.Request.Header.Get("User-Agent"),
		})
	}
	if len(logs) == 0 {
		w.abort(http.StatusInternalServerError, "empty target set")
		return nil, false
	}
	return logs, true
}

func (a *recycleAuditor) verifyDeletion(db *gorm.DB, resolvedTable string, normalizedIDs []string) {
	var remaining int64
	if err := db.Table(resolvedTable).Where("`"+a.rule.PrimaryKey+"` IN ?", normalizedIDs).Count(&remaining).Error; err != nil {
		a.work.abort(http.StatusInternalServerError, "delete verification failed")
		return
	}
	if remaining != 0 {
		a.work.abort(http.StatusInternalServerError, "delete was not complete")
	}
}
