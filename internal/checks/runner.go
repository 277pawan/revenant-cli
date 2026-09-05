package checks

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/pawan-bisht/revenant/internal/config"
)

const (
	StatusPass = "PASS"
	StatusFail = "FAIL"
)

// Result is one line in report.json and one line in the terminal.
type Result struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// RunAll walks cfg.Checks in order and dispatches by `type`.
//
// Add a new check = add a case here + a file next to this one.
// Unknown types fail the run (better than silently skipping).
func RunAll(conn *pgx.Conn, items []config.Check) ([]Result, error) {
	ctx := context.Background()
	var out []Result

	for i, item := range items {
		var (
			batch []Result
			err   error
		)

		switch item.Type {
		case "schema":
			batch, err = Schema(ctx, conn, item)
		case "row_count":
			batch, err = RowCount(ctx, conn, item)
		case "foreign_key":
			batch, err = ForeignKey(ctx, conn, item)
		case "golden_query":
			batch, err = Golden(ctx, conn, item)
		case "freshness":
			batch, err = Freshness(ctx, conn, item)
		default:
			return nil, fmt.Errorf("checks[%d]: unknown type %q", i, item.Type)
		}
		if err != nil {
			out = append(out, Result{
				Name:    fmt.Sprintf("check[%d]:%s", i, item.Type),
				Status:  StatusFail,
				Message: fmt.Sprintf("%s check error: %v", item.Type, err),
			})
			continue
		}
		out = append(out, batch...)
	}

	return out, nil
}
