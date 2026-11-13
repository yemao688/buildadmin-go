package repository

import (
	"fmt"
	"net/http/httptest"
	"os"
	"testing"

	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/querybuilder"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 关联字段 quickSearch 与 data_scope 的交互：点号条目走 EXISTS（条件只引用
// 主表 FK），主表 scope 必须始终生效——关联表匹配到 scope 外的行时结果为空；
// EXISTS 不放大行数，count 与 find 一致（无需 DISTINCT）。
func TestSearchJoinQuickSearchScope(t *testing.T) {
	prefix := fmt.Sprintf("sj_it_%d_", os.Getpid())
	db, config := testutil.OpenMySQL(t)
	config.Database.Prefix = prefix
	sqlDB, err := db.DB()
	require.NoError(t, err)
	enforcer := data_scope.NewClosureEnforcer(config)

	q := func(name string) string { return "`" + prefix + name + "`" }
	tables := []string{"admin", "admin_closure", "items"}
	for _, tableName := range tables {
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+q(tableName)).Error)
	}
	t.Cleanup(func() {
		for _, tableName := range tables {
			_ = db.Exec("DROP TABLE IF EXISTS " + q(tableName)).Error
		}
		_ = sqlDB.Close()
	})
	for _, stmt := range []string{
		"CREATE TABLE " + q("admin_closure") + " (ancestor_id INT NOT NULL, descendant_id INT NOT NULL, PRIMARY KEY (ancestor_id,descendant_id))",
		"CREATE TABLE " + q("admin") + " (id INT PRIMARY KEY, username VARCHAR(64) NOT NULL)",
		"CREATE TABLE " + q("items") + " (id INT PRIMARY KEY, admin_id INT NOT NULL, name VARCHAR(64) NOT NULL)",
	} {
		require.NoError(t, db.Exec(stmt).Error)
	}
	require.NoError(t, db.Exec("INSERT INTO "+q("admin_closure")+" VALUES (1,1),(1,2),(2,2)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q("admin")+" VALUES (1,'root'),(2,'child')").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q("items")+" VALUES (1,1,'alpha'),(2,2,'beta'),(3,2,'gamma')").Error)

	itemsTable := prefix + "items"
	table := TableInfo{
		TableName:        itemsTable,
		Key:              "id",
		QuickSearchField: "id,admin.username",
		SearchJoins: []querybuilder.SearchJoin{
			{Alias: "admin", Table: prefix + "admin", PK: "id", FK: "admin_id"},
		},
	}

	tests := []struct {
		name         string
		quickSearch  string
		actor        data_scope.Actor
		wantCount    int64
		wantRowCount int
	}{
		{name: "关联匹配 scope 内", quickSearch: "chi", actor: data_scope.Actor{AdminID: 2}, wantCount: 2, wantRowCount: 2},
		{name: "关联匹配 scope 外被主表 scope 挡住", quickSearch: "roo", actor: data_scope.Actor{AdminID: 2}, wantCount: 0, wantRowCount: 0},
		{name: "无限制可见 scope 外关联行", quickSearch: "roo", actor: data_scope.Actor{AdminID: 2, Unrestricted: true}, wantCount: 1, wantRowCount: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("GET", "/?quickSearch="+tt.quickSearch, nil)
			require.NoError(t, data_scope.SetActor(ctx, tt.actor))
			whereS, whereP, _, _, _, err := QueryBuilder(ctx, table, nil)
			require.NoError(t, err)
			require.NotEmpty(t, whereS)
			owner := data_scope.OwnerRef{TableAlias: itemsTable, Column: "admin_id"}

			var total int64
			require.NoError(t, enforcer.Scope(ctx, db.Table(itemsTable), owner).Where(whereS, whereP...).Count(&total).Error)
			rows := []map[string]interface{}{}
			require.NoError(t, enforcer.Scope(ctx, db.Table(itemsTable), owner).Where(whereS, whereP...).Find(&rows).Error)

			require.Equal(t, tt.wantCount, total)
			require.Len(t, rows, tt.wantRowCount)
			require.Equal(t, int(total), len(rows), "EXISTS 不应放大行数")
		})
	}
}
