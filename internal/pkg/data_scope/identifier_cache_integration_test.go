package data_scope

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"buildadmin-go/internal/pkg/testutil"
)

func TestBusinessIdentifierCacheMySQLHitExpiryAndInvalidation(t *testing.T) {
	db, _ := testutil.OpenMySQL(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	prefix := fmt.Sprintf("identifier_cache_%d_", os.Getpid())
	table := prefix + "items"
	quote := func(value string) string { return "`" + value + "`" }

	InvalidateBusinessIdentifierCache()
	now := time.Now()
	previousNow := businessIdentifierCacheNow
	businessIdentifierCacheNow = func() time.Time { return now }
	t.Cleanup(func() {
		businessIdentifierCacheNow = previousNow
		InvalidateBusinessIdentifierCache()
		_ = db.Exec("DROP TABLE IF EXISTS " + quote(table)).Error
		_ = sqlDB.Close()
	})

	require.NoError(t, db.Exec("CREATE TABLE "+quote(table)+" (id INT NOT NULL PRIMARY KEY, label VARCHAR(32) NOT NULL)").Error)
	resolved, err := ResolveBusinessTable(db, prefix, "items")
	require.NoError(t, err)
	require.Equal(t, table, resolved)
	require.NoError(t, ResolveBusinessColumn(db, table, "label", prefix))
	_, err = ResolveBusinessPrimaryKey(db, table, prefix)
	require.NoError(t, err)
	present, err := HasBusinessColumn(db, prefix, table, "label")
	require.NoError(t, err)
	require.True(t, present)

	// A warm cache keeps the schema result available even after the table is dropped.
	require.NoError(t, db.Exec("DROP TABLE "+quote(table)).Error)
	resolved, err = ResolveBusinessTable(db, prefix, "items")
	require.NoError(t, err)
	require.Equal(t, table, resolved)
	require.NoError(t, ResolveBusinessColumn(db, table, "label", prefix))
	_, err = ResolveBusinessPrimaryKey(db, table, prefix)
	require.NoError(t, err)
	present, err = HasBusinessColumn(db, prefix, table, "label")
	require.NoError(t, err)
	require.True(t, present)

	InvalidateBusinessIdentifierCache()
	_, err = ResolveBusinessTable(db, prefix, "items")
	require.Error(t, err)

	require.NoError(t, db.Exec("CREATE TABLE "+quote(table)+" (id INT NOT NULL, label VARCHAR(32) NOT NULL)").Error)
	InvalidateBusinessIdentifierCache()
	require.NoError(t, ResolveBusinessColumn(db, table, "id", prefix))
	_, err = ResolveBusinessPrimaryKey(db, table, prefix)
	require.Error(t, err)
	require.Error(t, ResolveBusinessColumn(db, table, "later", prefix))
	require.NoError(t, db.Exec("ALTER TABLE "+quote(table)+" ADD COLUMN later VARCHAR(32) NOT NULL DEFAULT ''").Error)
	// Missing-column and missing-primary-key results are cached until expiry.
	require.Error(t, ResolveBusinessColumn(db, table, "later", prefix))
	_, err = ResolveBusinessPrimaryKey(db, table, prefix)
	require.Error(t, err)
	require.NoError(t, db.Exec("ALTER TABLE "+quote(table)+" ADD PRIMARY KEY (id)").Error)

	now = now.Add(businessIdentifierCacheTTL + time.Nanosecond)
	require.NoError(t, ResolveBusinessColumn(db, table, "later", prefix))
	primary, err := ResolveBusinessPrimaryKey(db, table, prefix)
	require.NoError(t, err)
	require.Equal(t, "id", primary)
}
