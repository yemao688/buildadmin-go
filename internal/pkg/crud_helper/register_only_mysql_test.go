package crud_helper

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

// TestApplySkipsRegisterOnlyMySQL 验证 apply 对 registerOnly spec 静默跳过
// （返回 ApplySkipped，不建表不改表）——setup/migrate 尾部 apply 扫描
// crud_specs/ 时不会被受保护拒绝卡死。
func TestApplySkipsRegisterOnlyMySQL(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	prefix := fmt.Sprintf("ba_roa_%d_", os.Getpid())
	db.Config.NamingStrategy = schema.NamingStrategy{TablePrefix: prefix}
	cfg.Database.Prefix = prefix

	specPath := filepath.Join(t.TempDir(), "user.yaml")
	spec := "name: user\ncomment: 会员表\nregisterOnly: true\ndataScope:\n  mode: auto\n  reassignable: true\nfields:\n  - name: id\n    type: bigint\n    unsigned: true\n    primaryKey: true\n    autoIncrement: true\n    null: false\n    comment: ID\n  - name: admin_id\n    type: int\n    unsigned: true\n    null: false\n    comment: 管理员ID\n"
	require.NoError(t, os.WriteFile(specPath, []byte(spec), 0644))

	// plan 与 apply 都跳过，且不建表
	results, err := PlanSpecs(db, cfg, []string{specPath}, ApplyOptions{})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, ApplySkipped, results[0].Action)

	results, err = ApplySpecs(db, cfg, []string{specPath}, ApplyOptions{})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, ApplySkipped, results[0].Action)

	var exists int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", prefix+"user").Scan(&exists).Error)
	require.Zero(t, exists)
}
