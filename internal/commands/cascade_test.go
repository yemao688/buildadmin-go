package commands

import (
	"strings"
	"testing"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/data_scope"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// buildCascadeSyncJobs 单元测试：用构造的 LogRow 覆盖反向聚合、过滤与
// 失败关闭路径，不依赖真实 crud_log 数据。
func TestBuildCascadeSyncJobs(t *testing.T) {
	tests := []struct {
		name    string
		logs    []LogRow
		want    []CascadeJob
		wantErr bool
		errPart string
	}{
		{
			name: "反向聚合：两个子表 inheritFrom 同一主表 → 1 job 含 2 children",
			logs: []LogRow{
				{TableName: "user_money_log", TableJSON: `{
					"name": "user_money_log",
					"comment": "余额流水",
					"dataScope": {"mode": "auto", "inheritFrom": {"table": "user", "byColumn": "user_id"}}
				}`},
				{TableName: "order_recharge", TableJSON: `{
					"name": "order_recharge",
					"dataScope": {"inheritFrom": {"table": "user", "byColumn": "user_id"}}
				}`},
			},
			want: []CascadeJob{
				{ParentTable: "user", Children: []ChildRef{
					{ChildTable: "user_money_log", ByColumn: "user_id"},
					{ChildTable: "order_recharge", ByColumn: "user_id"},
				}},
			},
		},
		{
			name: "不同主表的子表产出多个 job",
			logs: []LogRow{
				{TableName: "user_money_log", TableJSON: `{
					"name": "user_money_log",
					"dataScope": {"inheritFrom": {"table": "user", "byColumn": "user_id"}}
				}`},
				{TableName: "seller_money_log", TableJSON: `{
					"name": "seller_money_log",
					"dataScope": {"inheritFrom": {"table": "seller_user", "byColumn": "seller_id"}}
				}`},
			},
			want: []CascadeJob{
				{ParentTable: "user", Children: []ChildRef{{ChildTable: "user_money_log", ByColumn: "user_id"}}},
				{ParentTable: "seller_user", Children: []ChildRef{{ChildTable: "seller_money_log", ByColumn: "seller_id"}}},
			},
		},
		{
			name: "key 大小写容错：DataScope/InheritFrom/ByColumn 均可解析",
			logs: []LogRow{{TableName: "user_money_log", TableJSON: `{
				"name": "user_money_log",
				"DataScope": {
					"Mode": "auto",
					"InheritFrom": {"Table": "user", "ByColumn": "user_id"}
				}
			}`}},
			want: []CascadeJob{
				{ParentTable: "user", Children: []ChildRef{{ChildTable: "user_money_log", ByColumn: "user_id"}}},
			},
		},
		{
			name: "无 dataScope 的行被过滤",
			logs: []LogRow{{TableName: "country", TableJSON: `{"name": "country"}`}},
			want: nil,
		},
		{
			name: "dataScope 存在但无 inheritFrom 的行被过滤（含历史 cascadeOwners 声明）",
			logs: []LogRow{{TableName: "user_money_log", TableJSON: `{
				"name": "user_money_log",
				"dataScope": {"mode": "auto", "ownerColumn": "admin_id",
					"cascadeOwners": [{"table": "user", "byColumn": "user_id"}]}
			}`}},
			want: nil,
		},
		{
			name:    "非法 JSON 行报错且携带 table_name",
			logs:    []LogRow{{TableName: "user_money_log", TableJSON: `{"name": "user_money_log", "dataScope": {`}},
			wantErr: true,
			errPart: `table "user_money_log"`,
		},
		{
			name: "多行混合：合法行产出任务，非法行整体报错",
			logs: []LogRow{
				{TableName: "user_money_log", TableJSON: `{
					"name": "user_money_log",
					"dataScope": {"inheritFrom": {"table": "user", "byColumn": "user_id"}}
				}`},
				{TableName: "broken", TableJSON: `not json`},
			},
			wantErr: true,
			errPart: `table "broken"`,
		},
		{
			name:    "inheritFrom 缺 byColumn → 不完整声明报错",
			logs:    []LogRow{{TableName: "user_money_log", TableJSON: `{"name": "user_money_log", "dataScope": {"inheritFrom": {"table": "user"}}}`}},
			wantErr: true,
			errPart: "incomplete inheritFrom",
		},
		{
			name:    "inheritFrom 缺 table → 不完整声明报错",
			logs:    []LogRow{{TableName: "user_money_log", TableJSON: `{"name": "user_money_log", "dataScope": {"inheritFrom": {"byColumn": "user_id"}}}`}},
			wantErr: true,
			errPart: "incomplete inheritFrom",
		},
		{
			name: "非法标识符：inheritFrom 主表名含点号被拒绝",
			logs: []LogRow{{TableName: "user_money_log", TableJSON: `{
				"name": "user_money_log",
				"dataScope": {"inheritFrom": {"table": "user.bad", "byColumn": "user_id"}}
			}`}},
			wantErr: true,
			errPart: "invalid",
		},
		{
			name: "非法标识符：子表名含空格被拒绝",
			logs: []LogRow{{TableName: "user_money_log", TableJSON: `{
				"name": "user money log",
				"dataScope": {"inheritFrom": {"table": "user", "byColumn": "user_id"}}
			}`}},
			wantErr: true,
			errPart: "invalid",
		},
		{
			name: "非法标识符：byColumn 含反引号被拒绝",
			logs: []LogRow{{TableName: "user_money_log", TableJSON: `{
				"name": "user_money_log",
				"dataScope": {"inheritFrom": {"table": "user", "byColumn": "user` + "`" + `id"}}
			}`}},
			wantErr: true,
			errPart: "invalid",
		},
		{
			name: "空 name（子表名缺失）的 JSON 被拒绝（非法标识符）",
			logs: []LogRow{{TableName: "user_money_log", TableJSON: `{
				"dataScope": {"inheritFrom": {"table": "user", "byColumn": "user_id"}}
			}`}},
			wantErr: true,
			errPart: "invalid",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			jobs, err := buildCascadeSyncJobs(test.logs)
			if test.wantErr {
				if err == nil {
					t.Fatalf("buildCascadeSyncJobs() error = nil, want error")
				}
				if test.errPart != "" && !strings.Contains(err.Error(), test.errPart) {
					t.Fatalf("buildCascadeSyncJobs() error = %v, want it to contain %q", err, test.errPart)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildCascadeSyncJobs() unexpected error: %v", err)
			}
			if len(jobs) != len(test.want) {
				t.Fatalf("jobs = %+v, want %+v", jobs, test.want)
			}
			for i := range test.want {
				if !sameJob(jobs[i], test.want[i]) {
					t.Errorf("jobs[%d] = %+v, want %+v", i, jobs[i], test.want[i])
				}
			}
		})
	}
}

// sameJob 比较两个 CascadeJob（Children 顺序敏感，聚合顺序由行顺序决定，
// 测试构造的行顺序即期望顺序）。
func sameJob(a, b CascadeJob) bool {
	if a.ParentTable != b.ParentTable || len(a.Children) != len(b.Children) {
		return false
	}
	for i := range a.Children {
		if a.Children[i] != b.Children[i] {
			return false
		}
	}
	return true
}

// TestRunCascadeJobRejectsInvalidIdentifiers 验证 runCascadeJob 在触达数据库
// 之前就拒绝非法标识符：校验先行，nil db 即可证明未发生任何执行。
func TestRunCascadeJobRejectsInvalidIdentifiers(t *testing.T) {
	cfg := &conf.Configuration{}
	invalid := []struct {
		parent string
		child  ChildRef
	}{
		{"user", ChildRef{ChildTable: "user_money log", ByColumn: "user_id"}},
		{"user", ChildRef{ChildTable: "user_money_log", ByColumn: "user.id"}},
		{"user", ChildRef{ChildTable: "user_money_log", ByColumn: "user`id"}},
		{"user", ChildRef{ChildTable: "user_money_log", ByColumn: ""}},
		{"user.bad", ChildRef{ChildTable: "user_money_log", ByColumn: "user_id"}},
		{"", ChildRef{ChildTable: "user_money_log", ByColumn: "user_id"}},
	}
	for _, tc := range invalid {
		if _, err := runCascadeJob(nil, cfg, tc.parent, tc.child); err == nil {
			t.Errorf("runCascadeJob(nil, cfg, %q, %+v) error = nil, want invalid identifier error", tc.parent, tc.child)
		} else if !strings.Contains(err.Error(), "invalid") {
			t.Errorf("runCascadeJob(nil, cfg, %q, %+v) error = %v, want invalid identifier error", tc.parent, tc.child, err)
		}
	}
}

// TestValidateCascadeJob 直接验证标识符规则与 data_scope 复用。
func TestValidateCascadeJob(t *testing.T) {
	valid := CascadeJob{ParentTable: "seller_user", Children: []ChildRef{{ChildTable: "seller_user_money_log", ByColumn: "user_id"}}}
	if err := validateCascadeJob(valid); err != nil {
		t.Fatalf("validateCascadeJob(%+v) unexpected error: %v", valid, err)
	}
	for _, bad := range []string{"", "a b", "a.b", "a`b", "a-b", "a\"b"} {
		job := valid
		job.Children[0].ByColumn = bad
		err := validateCascadeJob(job)
		if data_scope.IsValidIdentifier(bad) {
			if err != nil {
				t.Errorf("validateCascadeJob with byColumn %q: unexpected error %v", bad, err)
			}
		} else if err == nil {
			t.Errorf("validateCascadeJob with byColumn %q: error = nil, want error", bad)
		}
	}
	// 空 Children 的 job 通过校验（无可同步内容，Sync 汇总 0 行）。
	if err := validateCascadeJob(CascadeJob{ParentTable: "user"}); err != nil {
		t.Errorf("validateCascadeJob with no children: unexpected error %v", err)
	}
}

// newCascadeCrudLogTestDB 构造 sqlite 内存库 + ba_crud_log 表，用于
// loadCrudLogRows 的状态过滤单测（与 crud_helper 的 apply_test 同构）。
func newCascadeCrudLogTestDB(t *testing.T) (*gorm.DB, *conf.Configuration) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &conf.Configuration{}
	cfg.Database.Prefix = "ba_"
	if err := db.Exec("CREATE TABLE ba_crud_log (id INTEGER PRIMARY KEY AUTOINCREMENT, admin_id INTEGER NOT NULL, table_name TEXT NOT NULL, `table` BLOB, fields BLOB, status TEXT NOT NULL, comment TEXT, connection TEXT NOT NULL, sync INTEGER, create_time INTEGER)").Error; err != nil {
		t.Fatal(err)
	}
	return db, cfg
}

// insertCrudLogRow 按 (create_time, id) 递增顺序写入 crud_log 行。
func insertCrudLogRow(t *testing.T, db *gorm.DB, tableName, tableJSON, status string, id, createTime int) {
	t.Helper()
	if err := db.Exec("INSERT INTO ba_crud_log (id, admin_id, table_name, `table`, fields, status, connection, sync, create_time) VALUES (?, 1, ?, ?, '[]', ?, 'mysql', 0, ?)",
		id, tableName, tableJSON, status, createTime).Error; err != nil {
		t.Fatal(err)
	}
}

// inheritFromJSON 构造子表声明 inheritFrom 主表的 table JSON。
func inheritFromJSON(t *testing.T, childName, parentName string) string {
	t.Helper()
	return `{"name": "` + childName + `", "dataScope": {"mode": "auto",
		"inheritFrom": {"table": "` + parentName + `", "byColumn": "user_id"}}}`
}

// TestLoadCrudLogRowsSkipsConsumedSuccess 回归评审修复：子表 X 存在
// 成功A→成功B→delete(B) 序列时，X 的陈旧 success 行不得产出对账任务
// （模块已删除，子表可能已 DROP，旧实现会生成 job 并在 UPDATE 报 1146）。
func TestLoadCrudLogRowsSkipsConsumedSuccess(t *testing.T) {
	db, cfg := newCascadeCrudLogTestDB(t)
	insertCrudLogRow(t, db, "user_x_money_log", inheritFromJSON(t, "user_x_money_log", "user_x"), "success", 1, 1)
	insertCrudLogRow(t, db, "user_x_money_log", inheritFromJSON(t, "user_x_money_log", "user_x"), "success", 2, 2)
	insertCrudLogRow(t, db, "user_x_money_log", "", "delete", 3, 3)

	rows, err := loadCrudLogRows(db, cfg)
	if err != nil {
		t.Fatalf("loadCrudLogRows() error: %v", err)
	}
	for _, row := range rows {
		if row.TableName == "user_x_money_log" {
			t.Fatalf("loadCrudLogRows() kept consumed table %q (latest row is delete)", row.TableName)
		}
	}
	jobs, err := buildCascadeSyncJobs(rows)
	if err != nil {
		t.Fatalf("buildCascadeSyncJobs() error: %v", err)
	}
	for _, job := range jobs {
		if job.ParentTable == "user_x" {
			t.Fatalf("buildCascadeSyncJobs() produced job for consumed table user_x: %+v", job)
		}
	}
}

// TestLoadCrudLogRowsLatestSuccess 最新一行是 success 时正常产出任务；
// 同时覆盖最新行是 error 时跳过、delete 后重新生成的 success 保留。
func TestLoadCrudLogRowsLatestSuccess(t *testing.T) {
	db, cfg := newCascadeCrudLogTestDB(t)
	// user_ok_money_log：成功A→成功B，最新是 success → 保留并产出 job。
	insertCrudLogRow(t, db, "user_ok_money_log", inheritFromJSON(t, "user_ok_money_log", "user_ok"), "success", 1, 1)
	insertCrudLogRow(t, db, "user_ok_money_log", inheritFromJSON(t, "user_ok_money_log", "user_ok"), "success", 2, 2)
	// user_broken_money_log：success→error，最新不是 success → 跳过。
	insertCrudLogRow(t, db, "user_broken_money_log", inheritFromJSON(t, "user_broken_money_log", "user_broken"), "success", 3, 3)
	insertCrudLogRow(t, db, "user_broken_money_log", "", "error", 4, 4)
	// user_reborn_money_log：delete→success（删除后重新生成），最新 success → 保留。
	insertCrudLogRow(t, db, "user_reborn_money_log", "", "delete", 5, 5)
	insertCrudLogRow(t, db, "user_reborn_money_log", inheritFromJSON(t, "user_reborn_money_log", "user_reborn"), "success", 6, 6)

	rows, err := loadCrudLogRows(db, cfg)
	if err != nil {
		t.Fatalf("loadCrudLogRows() error: %v", err)
	}
	got := map[string]bool{}
	for _, row := range rows {
		got[row.TableName] = true
	}
	for _, want := range []string{"user_ok_money_log", "user_reborn_money_log"} {
		if !got[want] {
			t.Errorf("loadCrudLogRows() missing %q (latest row is success)", want)
		}
	}
	if got["user_broken_money_log"] {
		t.Errorf("loadCrudLogRows() kept %q (latest row is error)", "user_broken_money_log")
	}

	jobs, err := buildCascadeSyncJobs(rows)
	if err != nil {
		t.Fatalf("buildCascadeSyncJobs() error: %v", err)
	}
	jobParents := map[string]bool{}
	for _, job := range jobs {
		jobParents[job.ParentTable] = true
	}
	if !jobParents["user_ok"] || !jobParents["user_reborn"] {
		t.Fatalf("buildCascadeSyncJobs() jobs = %+v, want user_ok and user_reborn", jobs)
	}
	for _, job := range jobs {
		if job.ParentTable == "user_broken" {
			t.Fatalf("buildCascadeSyncJobs() produced job for user_broken: %+v", job)
		}
	}
}
