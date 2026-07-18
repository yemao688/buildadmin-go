package middleware

import (
	"testing"

	"github.com/gin-gonic/gin"
	"go-build-admin/app/pkg/data_scope"
	"gorm.io/gorm"
)

type ownerRecordingEnforcer struct{ ref data_scope.OwnerRef }

func (e *ownerRecordingEnforcer) Actor(_ *gin.Context) (data_scope.Actor, error) {
	return data_scope.Actor{AdminID: 1, Unrestricted: true}, nil
}

func (e *ownerRecordingEnforcer) Scope(_ *gin.Context, db *gorm.DB, ref data_scope.OwnerRef) *gorm.DB {
	e.ref = ref
	return db
}

func TestExtractOwnerIDUsesCustomColumn(t *testing.T) {
	owner, err := extractOwnerID(map[string]any{"operator_admin_id": int64(7)}, "operator_admin_id")
	if err != nil || owner != 7 {
		t.Fatalf("owner=%d err=%v", owner, err)
	}
	if _, err := extractOwnerID(map[string]any{"admin_id": int64(7)}, "operator_admin_id"); err == nil {
		t.Fatal("missing custom owner must fail closed")
	}
}

func TestSecurityScopeUsesCustomOwner(t *testing.T) {
	e := &ownerRecordingEnforcer{}
	ctx, _ := gin.CreateTestContext(nil)
	_ = securityScope(ctx, &gorm.DB{}, e, "ba_demo", "operator_admin_id")
	if e.ref.Column != "operator_admin_id" || e.ref.TableAlias != "ba_demo" {
		t.Fatalf("owner ref = %+v", e.ref)
	}
}
