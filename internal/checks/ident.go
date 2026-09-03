package checks

import (
	"fmt"
	"unicode"
)

// safeIdent allows only simple Postgres identifiers: letters, digits, underscore.
//
// This is the guard that lets us put a table name into SQL without quoting
// tricks. If you need "Order Items" or schema.table, extend this later
// instead of concatenating raw yaml into queries.
func safeIdent(name string) error {
	if name == "" {
		return fmt.Errorf("empty identifier")
	}
	for i, r := range name {
		ok := r == '_' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r))
		if !ok {
			return fmt.Errorf("unsafe identifier %q (use letters, digits, underscore)", name)
		}
	}
	return nil
}
