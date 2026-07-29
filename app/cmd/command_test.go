package cmd

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

func TestRegisterCommandsPropagateConstructionErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "migrate", args: []string{"migrate"}},
		{name: "crud:generate", args: []string{"crud:generate", "spec.yaml"}},
		{name: "crud:delete", args: []string{"crud:delete", "orders"}},
		{name: "crud:apply", args: []string{"crud:apply"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := &cobra.Command{Use: "root", SilenceErrors: true, SilenceUsage: true}
			want := errors.New("command construction failed")
			Register(root, func() (*Command, func(), error) {
				return nil, nil, want
			})
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
	Register(root, func() (*Command, func(), error) {
		return nil, nil, want
	})
	root.SetArgs([]string{"crud:apply", "--plan"})
	if err := root.Execute(); !errors.Is(err, want) {
		t.Fatalf("Execute() error = %v, want %v", err, want)
	}
}
