package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"os"
	"path/filepath"
	"testing"
)

func writeDiagnoseSpec(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spec.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExpectedMenuName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, spec, want string
	}{
		{
			name: "prefixed table",
			spec: "name: ops_banner\nfields:\n  id: {type: bigint, primaryKey: true}\n",
			want: "ops/banner",
		},
		{
			name: "single segment table",
			spec: "name: banner\nfields:\n  id: {type: bigint, primaryKey: true}\n",
			want: "banner",
		},
	}
	for _, tc := range cases {
		options, err := helper.LoadSpec(writeDiagnoseSpec(t, tc.spec))
		if err != nil {
			t.Fatal(err)
		}
		got, err := expectedMenuName(options)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("%s: menu name = %q, want %q", tc.name, got, tc.want)
		}
	}
}
