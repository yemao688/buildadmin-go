package commands

import (
	"errors"
	"testing"

	"buildadmin-go/internal/pkg/testutil"
)

// off 只关闭结构同步，不能跳过安装守卫：空库仍必须返回未安装错误。
func TestSchemaEnsureOffStillChecksInstall(t *testing.T) {
	t.Setenv("APP_AUTO_MIGRATE", "off")
	db, configuration := testutil.OpenFixtureDatabase(t, "schema_off_")
	if err := ensureSchema(db, configuration, ""); !errors.Is(err, errDatabaseNotInstalled) {
		t.Fatalf("ensureSchema(off) = %v, want errDatabaseNotInstalled", err)
	}
}
