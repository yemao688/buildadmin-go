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

// TestAuthRepositoryHasClosureSelfRow verifies the one-time per-request
// closure self-row verification backing H8: an admin with a self-row reports
// true, an admin without one (or an unknown id) reports false, and the check
// never errors on a well-formed closure table.
func TestAuthRepositoryHasClosureSelfRow(t *testing.T) {
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

	// Admin 3 has ancestor rows (it is 1's and 2's descendant) but no
	// self-row: the verification must report false so scope denies.
	ok, err = authM.HasClosureSelfRow(3)
	require.NoError(t, err)
	require.False(t, ok)

	// Unknown id: no self-row either.
	ok, err = authM.HasClosureSelfRow(99)
	require.NoError(t, err)
	require.False(t, ok)
}
