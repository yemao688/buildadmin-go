package model

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func dryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "test:test@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
	})
	if err != nil {
		t.Fatalf("failed to open dry-run db: %v", err)
	}
	return db
}

type captureLogger struct {
	logger.Interface
	sqls []string
}

func (l *captureLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	l.sqls = append(l.sqls, sql)
	l.Interface.Trace(ctx, begin, fc, err)
}

func TestLimitAdminIds_GeneratesInConditionForInt32(t *testing.T) {
	db := dryRunDB(t)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("dataLimitAdminIds", []int32{1, 2, 3})

	var logs []AdminLog
	stmt := db.Session(&gorm.Session{DryRun: true}).Model(&AdminLog{}).Scopes(LimitAdminIds(ctx)).Find(&logs).Statement
	sql := db.Dialector.Explain(stmt.SQL.String(), stmt.Vars...)

	if !strings.Contains(sql, "admin_id IN (1,2,3)") {
		t.Fatalf("expected admin_id IN condition, got: %s", sql)
	}
}

func TestLimitAdminIds_NoFilterForMissingOrInvalid(t *testing.T) {
	db := dryRunDB(t)
	cases := []struct {
		name string
		set  func(*gin.Context)
	}{
		{"nil", func(c *gin.Context) { c.Set("dataLimitAdminIds", nil) }},
		{"missing", func(c *gin.Context) {}},
		{"wrong-type", func(c *gin.Context) { c.Set("dataLimitAdminIds", []string{"1"}) }},
		{"empty", func(c *gin.Context) { c.Set("dataLimitAdminIds", []int32{}) }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			tc.set(ctx)

			var logs []AdminLog
			stmt := db.Session(&gorm.Session{DryRun: true}).Model(&AdminLog{}).Scopes(LimitAdminIds(ctx)).Find(&logs).Statement
			sql := db.Dialector.Explain(stmt.SQL.String(), stmt.Vars...)

			if strings.Contains(sql, "admin_id") {
				t.Fatalf("expected no admin_id filter for %s, got: %s", tc.name, sql)
			}
		})
	}
}

func TestGetAdminChildGroups_QueriesByUid(t *testing.T) {
	log := &captureLogger{Interface: logger.Default.LogMode(logger.Info)}
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "test:test@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		Logger:               log,
	})
	if err != nil {
		t.Fatalf("failed to open dry-run db: %v", err)
	}

	auth := NewAuthModel(db, nil, nil)
	_ = auth.GetAdminChildGroups(7)

	for _, sql := range log.sqls {
		if strings.Contains(sql, "admin_group_access") {
			if strings.Contains(sql, "WHERE id=") {
				t.Fatalf("expected uid column, got id: %s", sql)
			}
			if !strings.Contains(sql, "WHERE uid=") {
				t.Fatalf("expected uid = 7, got: %s", sql)
			}
			return
		}
	}
	t.Fatalf("expected admin_group_access query not captured: %v", log.sqls)
}
