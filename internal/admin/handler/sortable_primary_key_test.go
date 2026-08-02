package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	cErr "go-build-admin/internal/pkg/error"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type sortableInt64Row struct {
	OrderID int64 `gorm:"column:order_id;primaryKey"`
	Weigh   int32 `gorm:"column:weigh;not null"`
}

func (sortableInt64Row) TableName() string { return "sortable_int64_rows" }

type sortableStringRow struct {
	Token string `gorm:"column:token;primaryKey"`
	Weigh int32  `gorm:"column:weigh;not null"`
}

func (sortableStringRow) TableName() string { return "sortable_string_rows" }

type sortablePrimaryKeyModel struct {
	db         *gorm.DB
	table      string
	primaryKey string
}

func (m *sortablePrimaryKeyModel) DB() *gorm.DB  { return m.db }
func (m *sortablePrimaryKeyModel) Table() string { return m.table }
func (m *sortablePrimaryKeyModel) PrimaryKeyName() string {
	return m.primaryKey
}
func (m *sortablePrimaryKeyModel) Transaction(_ context.Context, fn func(*gorm.DB) error) error {
	return m.db.Transaction(fn)
}

func newSortablePrimaryKeyDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:sortable-primary-key-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func TestSortableDoesNotTruncateInt64CustomPrimaryKey(t *testing.T) {
	db := newSortablePrimaryKeyDB(t)
	require.NoError(t, db.AutoMigrate(&sortableInt64Row{}))
	ids := []int64{1<<40 + 1, 1<<40 + 2, 1<<40 + 3}
	require.NoError(t, db.Create([]sortableInt64Row{
		{OrderID: ids[0], Weigh: 10},
		{OrderID: ids[1], Weigh: 10},
		{OrderID: ids[2], Weigh: 10},
	}).Error)

	model := &sortablePrimaryKeyModel{db: db, table: "sortable_int64_rows", primaryKey: "order_id"}
	require.NoError(t, Sortable(sortableTestContext(), model, ids[0], ids[2], "up", "weigh,desc"))

	var rows []sortableInt64Row
	require.NoError(t, db.Order("order_id asc").Find(&rows).Error)
	weights := make(map[int64]int32, len(rows))
	for _, row := range rows {
		weights[row.OrderID] = row.Weigh
	}
	require.Equal(t, map[int64]int32{
		ids[0]: 10,
		ids[1]: 8,
		ids[2]: 9,
	}, weights)
}

func TestSortableUsesStringCustomPrimaryKeyColumn(t *testing.T) {
	db := newSortablePrimaryKeyDB(t)
	require.NoError(t, db.AutoMigrate(&sortableStringRow{}))
	require.NoError(t, db.Create([]sortableStringRow{
		{Token: "move", Weigh: 10},
		{Token: "target", Weigh: 10},
		{Token: "sibling", Weigh: 10},
	}).Error)

	model := &sortablePrimaryKeyModel{db: db, table: "sortable_string_rows", primaryKey: "token"}
	require.NoError(t, Sortable(sortableTestContext(), model, "move", "target", "up", "weigh,desc"))

	var rows []sortableStringRow
	require.NoError(t, db.Find(&rows).Error)
	weights := make(map[string]int32, len(rows))
	for _, row := range rows {
		weights[row.Token] = row.Weigh
	}
	require.Equal(t, map[string]int32{
		"move":    10,
		"target":  9,
		"sibling": 8,
	}, weights)
}

func TestBaseSortablePreservesLargeJSONPrimaryKey(t *testing.T) {
	db := newSortablePrimaryKeyDB(t)
	require.NoError(t, db.AutoMigrate(&sortableInt64Row{}))
	moveID := int64(1<<60 + 123)
	targetID := int64(1<<60 + 456)
	require.NoError(t, db.Create([]sortableInt64Row{
		{OrderID: moveID, Weigh: 10},
		{OrderID: targetID, Weigh: 10},
	}).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/sortable", strings.NewReader(fmt.Sprintf(`{"move":%d,"target":%d,"direction":"up","order":"weigh,desc"}`, moveID, targetID)))
	h := NewBase(&sortablePrimaryKeyModel{db: db, table: "sortable_int64_rows", primaryKey: "order_id"})
	h.Sortable(ctx)
	require.Equal(t, http.StatusOK, ctx.Writer.Status())

	var moved sortableInt64Row
	require.NoError(t, db.Where("order_id = ?", moveID).First(&moved).Error)
	require.Equal(t, int32(10), moved.Weigh)
}

func TestSortableRejectsUnsupportedPrimaryKeyShape(t *testing.T) {
	db := newSortablePrimaryKeyDB(t)
	model := &sortablePrimaryKeyModel{db: db, table: "sortable_int64_rows", primaryKey: "order_id"}
	err := Sortable(sortableTestContext(), model, []string{"invalid"}, int64(1), "up", "weigh,desc")
	var badRequest *cErr.Error
	require.ErrorAs(t, err, &badRequest)
	require.Equal(t, http.StatusBadRequest, badRequest.ErrorCode())
	require.Contains(t, badRequest.Error(), "Invalid move primary key")

	model.primaryKey = "order_id,tenant_id"
	err = Sortable(sortableTestContext(), model, int64(1), int64(2), "up", "weigh,desc")
	require.ErrorAs(t, err, &badRequest)
	require.Equal(t, http.StatusBadRequest, badRequest.ErrorCode())
	require.Contains(t, badRequest.Error(), "Unsupported primary key field")
}
