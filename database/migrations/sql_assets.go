package migrations

import (
	"bytes"
	"embed"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// migrationSQL contains only executable statements.  Statements which need
// schema inspection or data-dependent decisions remain in the Go callbacks.
// The {{name}} tokens are replaced with already validated, quoted identifiers.
//
//go:embed official/*.sql local/*.sql
var migrationSQL embed.FS

func execMigrationSQL(db *gorm.DB, asset string, values map[string]string) error {
	data, err := migrationSQL.ReadFile(asset)
	if err != nil {
		return fmt.Errorf("read migration SQL %s: %w", asset, err)
	}
	text := string(data)
	for key, value := range values {
		text = strings.ReplaceAll(text, "{{"+key+"}}", value)
	}
	if strings.Contains(text, "{{") {
		return fmt.Errorf("unresolved migration SQL token in %s", asset)
	}
	for _, statement := range bytes.Split([]byte(text), []byte(";")) {
		statement = bytes.TrimSpace(statement)
		if len(statement) == 0 {
			continue
		}
		if err := db.Exec(string(statement)).Error; err != nil {
			return fmt.Errorf("execute migration SQL %s: %w", asset, err)
		}
	}
	return nil
}
