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

Do not register migrations after `Migrations` has been called. The registry is
frozen at its first read.
