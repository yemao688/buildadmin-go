package repository

import (
	"fmt"
	"os"
	"testing"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/testutil"
	"buildadmin-go/internal/pkg/token"

	"github.com/stretchr/testify/require"
)

// 验证支撑 H8 的「每请求一次」闭包自引用行校验：有自引用行的管理员返回 true，
// 没有自引用行（或 id 未知）返回 false，且在结构良好的闭包表上永不报错。
func TestAuthRepositoryHasClosureSelfRow(t *testing.T) {
	t.Parallel()
	db, _ := testutil.OpenMySQL(t)
	prefix := fmt.Sprintf("ds_h8r_%d_", os.Getpid())
	closureTable := prefix + "admin_closure"
	q := func(name string) string { return "`" + name + "`" }
	db.Exec("DROP TABLE IF EXISTS " + q(closureTable))
	t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(closureTable)) })

	require.NoError(t, db.Exec("CREATE TABLE "+q(closureTable)+" (ancestor_id INT NOT NULL, descendant_id INT NOT NULL, PRIMARY KEY (ancestor_id, descendant_id))").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q(closureTable)+" (ancestor_id, descendant_id) VALUES (1,1),(1,2),(2,2)").Error)

	authM := NewAuthRepository(db, &token.TokenHelper{}, &conf.Configuration{Database: conf.Database{Prefix: prefix}})

	ok, err := authM.HasClosureSelfRow(1)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = authM.HasClosureSelfRow(2)
	require.NoError(t, err)
	require.True(t, ok)

	// 管理员 3 有祖先行（它是 1 和 2 的后代）但没有自引用行：
	// 校验必须返回 false，使 scope 拒绝。
	ok, err = authM.HasClosureSelfRow(3)
	require.NoError(t, err)
	require.False(t, ok)

	// 未知 id：同样没有自引用行。
	ok, err = authM.HasClosureSelfRow(99)
	require.NoError(t, err)
	require.False(t, ok)
}
