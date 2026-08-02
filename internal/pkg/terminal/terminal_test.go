package terminal

import (
	"go-build-admin/internal/pkg/token"
	"go-build-admin/internal/conf"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
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

type authStub struct {
	loggedIn        bool
	superAdmin      bool
	userID          int32
	superAdminCalls int
}

func (s *authStub) IsLogin(_ *gin.Context) (*token.Token, bool) {
	if !s.loggedIn {
		return nil, false
	}
	return &token.Token{UserID: s.userID}, true
}

func (s *authStub) IsSuperAdmin(_ int32) bool {
	s.superAdminCalls++
	return s.superAdmin
}

func TestExecAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name            string
		loggedIn        bool
		superAdmin      bool
		wantCreated     bool
		superAdminCalls int
	}{
		{name: "not logged in", wantCreated: false, superAdminCalls: 0},
		{name: "logged in non super administrator", loggedIn: true, wantCreated: false, superAdminCalls: 1},
		{name: "super administrator", loggedIn: true, superAdmin: true, wantCreated: true, superAdminCalls: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "executed")
			auth := &authStub{
				loggedIn:   tt.loggedIn,
				superAdmin: tt.superAdmin,
				userID:     1,
			}
			term := newTestTerminal()
			term.authM = auth
			term.config.Terminal.Commands["test"] = map[string]conf.Command{
				"create": {Command: "touch " + shellQuote(marker)},
			}

			writer := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(writer)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/?command=test.create", nil)
			term.Exec(ctx, true)

			_, err := os.Stat(marker)
			created := err == nil
			if err != nil && !os.IsNotExist(err) {
				t.Fatalf("stat execution marker: %v", err)
			}
			if created != tt.wantCreated {
				t.Fatalf("command created marker: %v, want %v", created, tt.wantCreated)
			}
			if auth.superAdminCalls != tt.superAdminCalls {
				t.Fatalf("IsSuperAdmin calls: %d, want %d", auth.superAdminCalls, tt.superAdminCalls)
			}
		})
	}
}
