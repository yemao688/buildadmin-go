package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/data_scope"
)

// writeSpec 向 specsDir 写入一个合法的最小 spec（name + id 主键 + dataScope）。
func writeSpec(t *testing.T, specsDir, name, dataScopeYAML string) {
	t.Helper()
	spec := "name: " + name + "\ncomment: test\n" + dataScopeYAML +
		"fields:\n" +
		"  - name: id\n    type: bigint\n    unsigned: true\n    primaryKey: true\n    autoIncrement: true\n    null: false\n    comment: ID\n"
	if err := os.WriteFile(filepath.Join(specsDir, name+".yaml"), []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestBuildCascadeSyncJobsFromSpecs 单元测试：用临时 specs 目录覆盖反向聚合、
// 过滤与失败关闭路径，不依赖 crud_log 数据。
func TestBuildCascadeSyncJobsFromSpecs(t *testing.T) {
	tests := []struct {
		name     string
		specs    map[string]string // 文件名 → dataScope YAML（写为合法 spec）
		broken   string            // 若非空，写入一个非法 spec 文件（触发 LoadSpec 失败）
		want     []CascadeJob
		wantErr  bool
		errPart  string
	}{
		{
			name: "反向聚合：两个子表 inheritFrom 同一主表 → 1 job 含 2 children",
			specs: map[string]string{
				"user_money_log": "dataScope:\n  mode: auto\n  inheritFrom:\n    table: user\n    byColumn: user_id\n",
				"order_recharge": "dataScope:\n  mode: auto\n  inheritFrom:\n    table: user\n    byColumn: user_id\n",
			},
			want: []CascadeJob{
				{ParentTable: "user", Children: []ChildRef{
					{ChildTable: "order_recharge", ByColumn: "user_id"},
					{ChildTable: "user_money_log", ByColumn: "user_id"},
				}},
			},
		},
		{
			name: "不同主表的子表产出多个 job",
			specs: map[string]string{
				"user_money_log":  "dataScope:\n  mode: auto\n  inheritFrom:\n    table: user\n    byColumn: user_id\n",
				"seller_money_log": "dataScope:\n  mode: auto\n  inheritFrom:\n    table: seller_user\n    byColumn: seller_id\n",
			},
			want: []CascadeJob{
				{ParentTable: "seller_user", Children: []ChildRef{{ChildTable: "seller_money_log", ByColumn: "seller_id"}}},
				{ParentTable: "user", Children: []ChildRef{{ChildTable: "user_money_log", ByColumn: "user_id"}}},
			},
		},
		{
			name: "无 dataScope 的 spec 被过滤",
			specs: map[string]string{
				"country": "",
			},
			want: nil,
		},
		{
			name: "dataScope 存在但无 inheritFrom 的 spec 被过滤",
			specs: map[string]string{
				"country": "dataScope:\n  mode: none\n",
			},
			want: nil,
		},
		{
			name: "非法 spec（缺字段）整体报错且携带文件名",
			specs: map[string]string{
				"user_money_log": "dataScope:\n  mode: auto\n  inheritFrom:\n    table: user\n    byColumn: user_id\n",
			},
			broken:  "broken.yaml",
			wantErr: true,
			errPart: "broken.yaml",
		},
		{
			name: "非法标识符：inheritFrom 主表名含点号被拒绝",
			specs: map[string]string{
				"user_money_log": "dataScope:\n  mode: auto\n  inheritFrom:\n    table: user.bad\n    byColumn: user_id\n",
			},
			wantErr: true,
			errPart: "invalid",
		},
		{
			name: "非法标识符：byColumn 含反引号被拒绝",
			specs: map[string]string{
				"user_money_log": "dataScope:\n  mode: auto\n  inheritFrom:\n    table: user\n    byColumn: \"user`id\"\n",
			},
			wantErr: true,
			errPart: "invalid",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			specsDir := t.TempDir()
			for fileName, dsYAML := range test.specs {
				writeSpec(t, specsDir, strings.TrimSuffix(fileName, ".yaml"), dsYAML)
			}
			if test.broken != "" {
				if err := os.WriteFile(filepath.Join(specsDir, test.broken), []byte("name: broken\n"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			jobs, err := buildCascadeSyncJobsFromSpecs(specsDir)
			if test.wantErr {
				if err == nil {
					t.Fatalf("buildCascadeSyncJobsFromSpecs() error = nil, want error")
				}
				if test.errPart != "" && !strings.Contains(err.Error(), test.errPart) {
					t.Fatalf("buildCascadeSyncJobsFromSpecs() error = %v, want it to contain %q", err, test.errPart)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildCascadeSyncJobsFromSpecs() unexpected error: %v", err)
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

// sameJob 比较两个 CascadeJob（Children 顺序敏感，聚合顺序由 spec 文件名字典
// 序决定，测试构造的文件名即期望顺序）。
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
