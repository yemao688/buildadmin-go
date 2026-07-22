package terminal

import (
	"go-build-admin/conf"
	"testing"
)

func newTestTerminal() *Terminal {
	cfg := &conf.Configuration{}
	cfg.Terminal.Commands = map[string]map[string]conf.Command{
		"npx": {
			"prettier": {Cwd: "web", Command: "npx prettier --write %s"},
		},
		"version": {
			"node": {Cwd: "", Command: "node -v"},
		},
	}
	return &Terminal{config: cfg}
}

func TestGetCommandPlaceholder(t *testing.T) {
	term := newTestTerminal()

	cmd, ok := term.GetCommand("npx.prettier", "./src/views/backend/test")
	if !ok {
		t.Fatal("npx.prettier should resolve")
	}
	want := "npx prettier --write './src/views/backend/test'"
	if cmd.Command != want {
		t.Fatalf("got %q, want %q", cmd.Command, want)
	}
}

func TestGetCommandPlaceholderEscapesInjection(t *testing.T) {
	term := newTestTerminal()

	cmd, ok := term.GetCommand("npx.prettier", "./x'; rm -rf /; echo '")
	if !ok {
		t.Fatal("npx.prettier should resolve")
	}
	want := `npx prettier --write './x'\''; rm -rf /; echo '\'''`
	if cmd.Command != want {
		t.Fatalf("got %q, want %q", cmd.Command, want)
	}
}

func TestGetCommandPlaceholderTildeSeparatedArgs(t *testing.T) {
	term := newTestTerminal()
	term.config.Terminal.Commands["fmt"] = map[string]conf.Command{
		"two": {Cwd: "", Command: "echo %s %s"},
	}

	cmd, ok := term.GetCommand("fmt.two", "a~~b c")
	if !ok {
		t.Fatal("fmt.two should resolve")
	}
	want := "echo 'a' 'b c'"
	if cmd.Command != want {
		t.Fatalf("got %q, want %q", cmd.Command, want)
	}
}

func TestGetCommandWithoutPlaceholderIgnoresExtend(t *testing.T) {
	term := newTestTerminal()

	cmd, ok := term.GetCommand("version.node", "anything")
	if !ok {
		t.Fatal("version.node should resolve")
	}
	if cmd.Command != "node -v" {
		t.Fatalf("got %q", cmd.Command)
	}
}

func TestGetCommandUnknownKey(t *testing.T) {
	term := newTestTerminal()
	if _, ok := term.GetCommand("npx.missing", ""); ok {
		t.Fatal("unknown sub key must not resolve")
	}
	if _, ok := term.GetCommand("npx", ""); ok {
		t.Fatal("key without dot must not resolve")
	}
}
