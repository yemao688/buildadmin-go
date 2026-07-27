# Business migrations

Add business migrations as Go files in this package. Register them from `init`:

```go
package business

import (
	"go-build-admin/conf"
	"gorm.io/gorm"
)

func init() {
	Register(Migration{
		Sequence: 1,
		ID:       "shop-orders",
		Revision: 1,
		Up: func(db *gorm.DB, config *conf.Configuration) error {
			// Create or alter only tables owned by this business migration.
			return nil
		},
		VerifyBaseline: func(db *gorm.DB, config *conf.Configuration) error {
			return nil
		},
		VerifySchema: func(db *gorm.DB, config *conf.Configuration) error {
			return nil
		},
		VerifyData: func(db *gorm.DB, config *conf.Configuration) error {
			return nil
		},
	})
}
```

`Up` must be idempotent and deduplicate by business keys, not by incidental row
positions. Do not assume a `ba_` prefix; use the configured prefix and
`internal/core.TableName` when constructing table names. Do not modify data in
official or local tables unless that data is owned by the business migration.

`VerifyBaseline` is an apply-time, one-shot contract: it runs after `Up`, is
retried when applying a failed migration, and does not run again after the
ledger record is complete. It may therefore assert the exact schema baseline
created by that migration. `VerifySchema` and `VerifyData` are standing
invariants: they run on every `migrate`, and their predicates must remain
compatible with valid business changes.

The business track is the final source of schema shape. A business migration
may override framework core columns after establishing the framework baseline,
but it then owns the resulting contract. For example, changing an amount to
`decimal` is a domain change and requires corresponding changes to the app
models and all arithmetic that reads or writes the amount; changing the column
alone is unsafe.

After business migrations, these standing checks still apply:

- local `VerifySchema` checks each completed local migration's standing schema
  contract; local `VerifyUpgradeData` checks its standing data contract.
- `local.VerifyCurrent` checks cross-table ownership, closure self-rows,
  security seed identity, and rejection of known legacy installer rules.
- `official.ValidateCurrentSchema` checks the current official schema
  requirements, including `user_rule.no_login_valid` and the current rule
  enum values.

Do not register migrations after `Migrations` has been called. The registry is
frozen at its first read.
