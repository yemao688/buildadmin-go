package data_scope

import "fmt"

// ValidateTablePrefix validates the same prefix grammar used by migrations.
// Keep this rule synchronized with migration prefix validation; this runtime
// lane deliberately does not import or modify the migration package.
func ValidateTablePrefix(prefix string) error {
	for _, r := range prefix {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return fmt.Errorf("%w: invalid table prefix %q", ErrInvalidIdentifier, prefix)
		}
	}
	return nil
}
