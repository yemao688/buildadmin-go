package model

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
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

func TestLimitAdminIds_GeneratesInConditionForInt32(t *testing.T) {
	db := dryRunDB(t)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("dataLimitAdminIds", []int32{1, 2, 3})

	var attachments []Attachment
	stmt := db.Session(&gorm.Session{DryRun: true}).Model(&Attachment{}).Scopes(LimitAdminIds(ctx)).Find(&attachments).Statement
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

			var attachments []Attachment
			stmt := db.Session(&gorm.Session{DryRun: true}).Model(&Attachment{}).Scopes(LimitAdminIds(ctx)).Find(&attachments).Statement
			sql := db.Dialector.Explain(stmt.SQL.String(), stmt.Vars...)

			if strings.Contains(sql, "admin_id") {
				t.Fatalf("expected no admin_id filter for %s, got: %s", tc.name, sql)
			}
		})
	}
}
