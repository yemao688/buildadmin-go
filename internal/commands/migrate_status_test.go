package commands

import (
	"os"
	"path/filepath"
	"testing"

	"buildadmin-go/internal/migrations"
)

func TestMigrateStatusLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		summary migrations.StatusSummary
		specs   int
		want    string
	}{
		{
			// 全绿：每轨 done/total 相等，pending/incomplete 为 0。
			name: "全部完成",
			summary: migrations.StatusSummary{
				State:     migrations.InstallStrictUpgrade,
				Official:  migrations.TrackSummary{Done: 12, Total: 12},
				Framework: migrations.TrackSummary{Done: 1, Total: 1},
				Business:  migrations.TrackSummary{Done: 3, Total: 3},
			},
			specs: 6,
			want:  "installed  state=strict_upgrade  migrations=12/12 pending=0 incomplete=0  framework=1/1 pending=0 incomplete=0  business=3/3 pending=0 incomplete=0  specs=6",
		},
		{
			// 有待执行与未完成记录时，计数必须出现在对应轨道段落里。
			name: "有待执行与未完成",
			summary: migrations.StatusSummary{
				State:     migrations.InstallStrictUpgrade,
				Official:  migrations.TrackSummary{Done: 12, Total: 12},
				Framework: migrations.TrackSummary{Done: 1, Total: 1, Incomplete: 1},
				Business:  migrations.TrackSummary{Done: 1, Total: 3, Pending: 2},
			},
			specs: 0,
			want:  "installed  state=strict_upgrade  migrations=12/12 pending=0 incomplete=0  framework=1/1 pending=0 incomplete=1  business=1/3 pending=2 incomplete=0  specs=0",
		},
		{
			// 未安装时不能自称 installed，三轨保持零值。
			name:    "未安装",
			summary: migrations.StatusSummary{State: migrations.InstallFresh},
			want:    "not-installed  state=fresh  migrations=0/0 pending=0 incomplete=0  framework=0/0 pending=0 incomplete=0  business=0/0 pending=0 incomplete=0  specs=0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := formatMigrateStatus(tt.summary, tt.specs); got != tt.want {
				t.Fatalf("formatMigrateStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCountSpecFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"a.yaml", "b.yaml", "notes.md"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("name: x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := countSpecFiles(dir); err != nil || got != 2 {
		t.Fatalf("countSpecFiles() = %d, %v; want 2, nil", got, err)
	}
	if got, err := countSpecFiles(""); err != nil || got != 0 {
		t.Fatalf("countSpecFiles(\"\") = %d, %v; want 0, nil", got, err)
	}
}
