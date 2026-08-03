package commands

import (
	"errors"
	"testing"

	"buildadmin-go/internal/conf"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
)

func TestRegisterCommandsPropagateConstructionErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "migrate", args: []string{"migrate"}},
		{name: "migrate run", args: []string{"migrate", "run"}},
		{name: "migrate rollback", args: []string{"migrate", "rollback"}},
		{name: "migrate rollback steps", args: []string{"migrate", "rollback", "--steps", "2"}},
		{name: "migrate breakpoint", args: []string{"migrate", "breakpoint"}},
		{name: "migrate breakpoint set", args: []string{"migrate", "breakpoint", "set", "1"}},
		{name: "migrate breakpoint clear", args: []string{"migrate", "breakpoint", "clear"}},
		{name: "migrate breakpoint list", args: []string{"migrate", "breakpoint", "list"}},
		{name: "crud:generate", args: []string{"crud:generate", "spec.yaml"}},
		{name: "crud:delete", args: []string{"crud:delete", "orders"}},
		{name: "crud:apply", args: []string{"crud:apply"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := &cobra.Command{Use: "root", SilenceErrors: true, SilenceUsage: true}
			want := errors.New("command construction failed")
			registerCommands(root, func(*conf.Configuration, *lumberjack.Logger, *zap.Logger) (*Command, func(), error) {
				return nil, nil, want
			}, nil)
			root.SetArgs(test.args)

			if err := root.Execute(); !errors.Is(err, want) {
				t.Fatalf("Execute() error = %v, want %v", err, want)
			}
		})
	}
}

func TestRegisterCrudApplyAcceptsPlanFlag(t *testing.T) {
	root := &cobra.Command{Use: "root", SilenceErrors: true, SilenceUsage: true}
	want := errors.New("command construction failed")
	registerCommands(root, func(*conf.Configuration, *lumberjack.Logger, *zap.Logger) (*Command, func(), error) {
		return nil, nil, want
	}, nil)
	root.SetArgs([]string{"crud:apply", "--plan"})
	if err := root.Execute(); !errors.Is(err, want) {
		t.Fatalf("Execute() error = %v, want %v", err, want)
	}
}
