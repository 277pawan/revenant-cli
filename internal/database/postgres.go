// Package database is the *only* place that knows about pgx.
//
// Checks receive a *pgx.Conn and run SQL. If we later add MySQL, you
// introduce an interface here — you do not sprinkle pgx calls through checks.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const connectTimeout = 10 * time.Second

// Connect opens a single Postgres connection.
//
// Phase 1 uses one connection (plenty for a handful of checks). A pool
// (pgxpool) is only worth it when we restore RDS and run many queries.
func Connect(connectionString string) (*pgx.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	conn, err := pgx.Connect(ctx, connectionString)
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}

	if err := conn.Ping(ctx); err != nil {
		conn.Close(ctx)
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	return conn, nil
}
