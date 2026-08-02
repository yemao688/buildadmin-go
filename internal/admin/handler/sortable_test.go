package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type sortableTestRow struct {
	ID      int32 `gorm:"column:id;primaryKey"`
	AdminID int32 `gorm:"column:admin_id;not null"`
	Weigh   int32 `gorm:"column:weigh;not null"`
}

func (sortableTestRow) TableName() string { return "sortable_test_rows" }

type sortableTestModel struct {
	db      *gorm.DB
	ownerID *int32
}

func (m *sortableTestModel) DB() *gorm.DB  { return m.db }
func (m *sortableTestModel) Table() string { return "sortable_test_rows" }
func (m *sortableTestModel) Transaction(_ context.Context, fn func(*gorm.DB) error) error {
	return m.db.Transaction(fn)
}
func (m *sortableTestModel) ScopeDB(_ *gin.Context, db *gorm.DB) *gorm.DB {
	if m.ownerID == nil {
		return db
	}
	return db.Where("admin_id = ?", *m.ownerID)
}

type sortableSQLCounter struct {
	logger.Interface
	selects int
	updates int
}

func (l *sortableSQLCounter) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	switch {
	case strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "SELECT"):
		l.selects++
	case strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "UPDATE"):
		l.updates++
	}
}

func newSortableTestDB(t *testing.T) (*gorm.DB, *sortableSQLCounter) {
	t.Helper()
	counter := &sortableSQLCounter{Interface: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(sqlite.Open("file:sortable-test-"+time.Now().Format("150405.000000000")+"?mode=memory&cache=shared"), &gorm.Config{Logger: counter})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&sortableTestRow{}))
	counter.selects = 0
	counter.updates = 0
	return db, counter
}

func sortableTestContext() *gin.Context {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/sortable", nil)
	return ctx
}

func sortableWeights(t *testing.T, db *gorm.DB) map[int32]int32 {
	t.Helper()
	var rows []sortableTestRow
	require.NoError(t, db.Order("id asc").Find(&rows).Error)
	weights := make(map[int32]int32, len(rows))
	for _, row := range rows {
		weights[row.ID] = row.Weigh
	}
	return weights
}

func TestSortableBatchUpdatePreservesEqualWeightSemantics(t *testing.T) {
	db, counter := newSortableTestDB(t)
	require.NoError(t, db.Create([]sortableTestRow{
		{ID: 1, AdminID: 1, Weigh: 10},
		{ID: 2, AdminID: 1, Weigh: 10},
		{ID: 3, AdminID: 1, Weigh: 10},
		{ID: 4, AdminID: 1, Weigh: 10},
		{ID: 5, AdminID: 1, Weigh: 12},
		{ID: 6, AdminID: 1, Weigh: 9},
	}).Error)

	model := &sortableTestModel{db: db}
	require.NoError(t, Sortable(sortableTestContext(), model, 1, 3, "up", "weigh,desc"))
	selects, updates := counter.selects, counter.updates
	require.Equal(t, map[int32]int32{
		1: 9,
		2: 7,
		3: 8,
		4: 10,
		5: 12,
		6: 5,
	}, sortableWeights(t, db))

	// The optimized path performs two initial reads, one batch read, one shift
	// update, and one CASE update, independent of the number of equal siblings.
	require.Equal(t, 3, selects)
	require.Equal(t, 2, updates)
}

func TestSortableBatchUpdatePreservesScope(t *testing.T) {
	db, _ := newSortableTestDB(t)
	require.NoError(t, db.Create([]sortableTestRow{
		{ID: 1, AdminID: 1, Weigh: 10},
		{ID: 2, AdminID: 1, Weigh: 10},
		{ID: 3, AdminID: 2, Weigh: 10},
		{ID: 4, AdminID: 1, Weigh: 9},
	}).Error)

	ownerID := int32(1)
	model := &sortableTestModel{db: db, ownerID: &ownerID}
	require.NoError(t, Sortable(sortableTestContext(), model, 4, 1, "up", "weigh,desc"))
	require.Equal(t, map[int32]int32{
		1: 8,
		2: 10,
		3: 10,
		4: 9,
	}, sortableWeights(t, db))
}
