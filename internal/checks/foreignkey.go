package checks

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/pawan-bisht/revenant/internal/config"
)

// ForeignKey is a stub so you can finish this check without touching the runner.
//
// YAML you will support (already parsed on config.Check):
//
//	- type: foreign_key
//	  table: order_items
//	  references: orders
//
// Implementation sketch (replace the body below):
//
//  1. Confirm both tables exist (reuse tableExists from schema.go, or
//     un-export it into a shared helper if you prefer).
//  2. Ask the catalog whether a FK actually exists:
//
//     SELECT COUNT(*)
//     FROM information_schema.table_constraints tc
//     JOIN information_schema.referential_constraints rc
//       ON tc.constraint_name = rc.constraint_name
//      AND tc.constraint_schema = rc.constraint_schema
//     JOIN information_schema.constraint_column_usage ccu
//       ON rc.unique_constraint_name = ccu.constraint_name
//     WHERE tc.constraint_type = 'FOREIGN KEY'
//       AND tc.table_schema = 'public'
//       AND tc.table_name = $1          -- child table
//       AND ccu.table_name = $2         -- parent table
//
//  3. Optionally prove *data* integrity, not just that the constraint exists:
//
//     SELECT COUNT(*) FROM order_items oi
//     LEFT JOIN orders o ON o.id = oi.order_id
//     WHERE o.id IS NULL;
//
//     That needs the FK column name — either add it to yaml
//     (`column: order_id`) or look it up in information_schema.key_column_usage.
//
// Until you implement it, listing foreign_key in revenant.yaml will FAIL
// the plan on purpose (honest report, not a silent skip).
func ForeignKey(ctx context.Context, conn *pgx.Conn, item config.Check) ([]Result, error) {
	_ = ctx
	_ = conn

	if item.Table == "" || item.References == "" {
		return nil, fmt.Errorf("foreign_key check needs table and references")
	}

	return []Result{{
		Name:    fmt.Sprintf("foreign_key:%s->%s", item.Table, item.References),
		Status:  StatusFail,
		Message: "foreign_key check is not implemented yet (see internal/checks/foreignkey.go)",
	}}, nil
}
