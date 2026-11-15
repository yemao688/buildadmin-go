package commands

import "testing"

// 独立生成器入口只暴露 CRUD 命令；--skip-compile-check 必须同时存在于
// generate 与 delete，供旧产物阻塞编译时批量重生成。
func TestCrudOnlyRootCommandRegistersCrudCommands(t *testing.T) {
	t.Parallel()
	root := newCrudOnlyRootCommand(crudOnlyBootstrap)

	want := map[string]bool{
		"crud:generate": false,
		"crud:delete":   false,
		"crud:apply":    false,
		"crud:validate": false,
	}
	for _, command := range root.Commands() {
		if _, ok := want[command.Name()]; ok {
			want[command.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("cmd/crudgen root is missing %q", name)
		}
	}

	for _, name := range []string{"crud:generate", "crud:delete"} {
		command, _, err := root.Find([]string{name})
		if err != nil {
			t.Fatalf("find %s: %v", name, err)
		}
		if command.Flags().Lookup("skip-compile-check") == nil {
			t.Fatalf("%s is missing --skip-compile-check", name)
		}
	}
}
