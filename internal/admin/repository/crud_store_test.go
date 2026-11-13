package repository

import (
	"net/http/httptest"
	"testing"

	persistence "buildadmin-go/internal/pkg/persistence"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type crudStoreTestRow struct {
	ID    int32
	Name  string
	Weigh int32
}

func newCRUDStoreTest(t *testing.T) (*CRUDStore, *gin.Context, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE store_items (id INTEGER PRIMARY KEY, name TEXT, weigh INTEGER)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO store_items (id, name, weigh) VALUES (1, 'a', 10)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO store_items (id, name, weigh) VALUES (2, 'b', 20)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO store_items (id, name, weigh) VALUES (3, 'c', 30)`).Error)
	base := persistence.NewBaseModel("store_items", "id", "", db)
	store := NewCRUDStore(&base, func() any { return &crudStoreTestRow{} }, nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	return &store, ctx, db
}

func TestCRUDStoreGetOneScoped(t *testing.T) {
	store, ctx, _ := newCRUDStoreTest(t)
	row, err := store.GetOneScoped(ctx, 2)
	require.NoError(t, err)
	got, ok := row.(*crudStoreTestRow)
	require.True(t, ok)
	require.Equal(t, "b", got.Name)

	_, err = store.GetOneScoped(ctx, 99)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestCRUDStoreUpdateFieldScopedTreatsVisibleNoopAsSuccess(t *testing.T) {
	store, ctx, _ := newCRUDStoreTest(t)
	// SQLite 对“写回同值”通常返回 1 行；MySQL 可能返回 0 行。两种都应视为成功。
	rows, visible, err := store.UpdateFieldScoped(ctx, 1, "name", "a")
	require.NoError(t, err)
	require.Contains(t, []int64{0, 1}, rows)
	require.True(t, visible)

	rows, visible, err = store.UpdateFieldScoped(ctx, 1, "name", "changed")
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)
	require.True(t, visible)

	_, visible, err = store.UpdateFieldScoped(ctx, 99, "name", "missing")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.False(t, visible)
}

func TestCRUDStoreSortWeighScoped(t *testing.T) {
	store, ctx, db := newCRUDStoreTest(t)
	require.NoError(t, store.SortWeighScoped(ctx, "1", "2", "down", "asc"))

	var rows []crudStoreTestRow
	require.NoError(t, db.Table("store_items").Order("id").Find(&rows).Error)
	require.Equal(t, []int32{20, 19, 30}, []int32{rows[0].Weigh, rows[1].Weigh, rows[2].Weigh})
}
