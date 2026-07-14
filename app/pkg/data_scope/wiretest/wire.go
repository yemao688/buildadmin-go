//go:build wireinject
// +build wireinject

package wiretest

//go:generate go run github.com/google/wire/cmd/wire

import (
	"go-build-admin/app/pkg"
	"go-build-admin/app/pkg/data_scope"

	"github.com/google/wire"
)

// generatedScopedModel mirrors the constructor shape emitted by the CRUD
// template: a scoped model receives the Enforcer explicitly.
type generatedScopedModel struct {
	Enforcer data_scope.Enforcer
}

func newGeneratedScopedModel(enforcer data_scope.Enforcer) *generatedScopedModel {
	return &generatedScopedModel{Enforcer: enforcer}
}

// Initialize proves the application package provider graph can resolve the
// Enforcer interface required by a generated scoped model.
func Initialize() *generatedScopedModel {
	panic(wire.Build(pkg.ProviderSet, newGeneratedScopedModel))
}
