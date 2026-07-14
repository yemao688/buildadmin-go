package data_scope

import (
	"fmt"
)

// ResolveOptions customizes policy resolution. It is intentionally small;
// the contract lane avoids database access, but callers may supply a
// ValidateOwnerColumn callback to prove that a required owner column exists
// and is compatible in later lanes.
type ResolveOptions struct {
	// AllowNoneWithAdminID permits the high-friction explicit override
	// described by the contract: a resource with an admin_id column may still
	// be declared ModeNone if the caller records the override elsewhere.
	AllowNoneWithAdminID bool
	// ValidateOwnerColumn is an optional callback used by ModeRequired.
	// Returning an error fails resolution with that error.
	ValidateOwnerColumn func(column string) error
}

// NormalizeMode coerces an empty mode to ModeAuto and returns the empty
// string for unknown values so that callers can fail-closed.
func NormalizeMode(m Mode) Mode {
	switch m {
	case "":
		return ModeAuto
	case ModeAuto, ModeRequired, ModeNone:
		return m
	default:
		return ""
	}
}

// ResolveConfig turns a persisted Config (which may be nil) into an effective
// Resolved policy. Missing config resolves to ModeAuto, never ModeNone, so
// legacy resources keep the existing convention-based behavior.
func ResolveConfig(cfg *Config, hasAdminID bool) (Resolved, error) {
	return ResolveConfigWithOptions(cfg, hasAdminID, ResolveOptions{})
}

// ResolveConfigWithOptions is the configurable variant of ResolveConfig.
// It implements the contract rules:
//   - nil/empty Config => ModeAuto.
//   - ModeAuto + exact admin_id => hierarchy/admin_id/assign-on-create.
//   - ModeAuto + no admin_id => explicit ModeNone.
//   - ModeRequired validates the configured owner column.
//   - ModeNone + admin_id requires AllowNoneWithAdminID.
//   - admin.id must be explicit: ModeRequired + OwnerColumn="id" +
//     AssignOnCreate=false or nil.
func ResolveConfigWithOptions(cfg *Config, hasAdminID bool, opts ResolveOptions) (Resolved, error) {
	if cfg == nil {
		cfg = &Config{Mode: ModeAuto}
	}

	mode := NormalizeMode(cfg.Mode)
	if mode == "" {
		return Resolved{}, fmt.Errorf("%w: %q", ErrInvalidMode, cfg.Mode)
	}

	resolved := Resolved{Mode: mode, Source: "config"}

	switch mode {
	case ModeAuto:
		if hasAdminID {
			if cfg.AssignOnCreate != nil && !*cfg.AssignOnCreate {
				return Resolved{}, fmt.Errorf("%w: auto+admin_id requires assign-on-create=true", ErrInvalidConfig)
			}
			resolved.OwnerColumn = "admin_id"
			resolved.OwnerGoField = "AdminID"
			resolved.AssignOnCreate = true
			resolved.Source = "auto:admin_id"
		} else {
			resolved.Mode = ModeNone
			resolved.Source = "auto:none"
		}

	case ModeRequired:
		col := cfg.OwnerColumn
		if col == "" {
			return Resolved{}, fmt.Errorf("%w: owner column is required", ErrInvalidOwnerColumn)
		}
		if err := ValidateIdentifier(col); err != nil {
			return Resolved{}, fmt.Errorf("%w: %w", ErrInvalidOwnerColumn, err)
		}
		if opts.ValidateOwnerColumn != nil {
			if err := opts.ValidateOwnerColumn(col); err != nil {
				return Resolved{}, err
			}
		}
		resolved.OwnerColumn = col
		resolved.OwnerGoField = deriveGoField(col)
		if col == "id" && cfg.AssignOnCreate != nil && *cfg.AssignOnCreate {
			return Resolved{}, fmt.Errorf("%w: admin.id cannot assign on create", ErrInvalidConfig)
		}
		if cfg.AssignOnCreate != nil {
			resolved.AssignOnCreate = *cfg.AssignOnCreate
		}

	case ModeNone:
		if hasAdminID && !opts.AllowNoneWithAdminID {
			return Resolved{}, fmt.Errorf("%w: admin_id present with mode none requires explicit override", ErrInvalidMode)
		}
	}

	return resolved, nil
}
