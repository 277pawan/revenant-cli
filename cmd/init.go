package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/pawan-bisht/revenant/internal/database"
	"github.com/pawan-bisht/revenant/internal/discover"
)

var (
	initOut    string // --output  where to write yaml
	initPlan   string // --plan
	initSchema string // --schema  Postgres schema to scan (usually public)
	initForce  bool   // --force   overwrite an existing file
)

// initCmd scaffolds revenant.yaml from a live database.
//
// This is the "git init" of Revenant: it writes a starter config you then
// edit. It never claims the database is healthy — that is still `verify`.
var initCmd = &cobra.Command{
	Use:          "init",
	Short:        "Scan a database and write a starter revenant.yaml",
	SilenceUsage: true,
	RunE:         runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(&initOut, "output", "o", "revenant.yaml", "path to write")
	initCmd.Flags().StringVar(&initPlan, "plan", "local-demo", "plan name to put in the yaml")
	initCmd.Flags().StringVar(&initSchema, "schema", "public", "Postgres schema to scan")
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite the output file if it already exists")
}

func runInit(cmd *cobra.Command, args []string) error {
	_ = godotenv.Load()

	connStr := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if connStr == "" {
		return fmt.Errorf("DATABASE_URL is not set (export it or put it in .env)")
	}

	if !initForce {
		if _, err := os.Stat(initOut); err == nil {
			return fmt.Errorf("%s already exists (pass --force to overwrite)", initOut)
		}
	}

	db, err := database.ConnectURL(connStr)
	if err != nil {
		return err
	}
	defer db.Close(cmd.Context())

	snap, err := discover.Schema(cmd.Context(), db.Conn(), initSchema)
	if err != nil {
		return err
	}

	body, err := discover.YAML(initPlan, snap)
	if err != nil {
		return err
	}

	if err := os.WriteFile(initOut, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", initOut, err)
	}

	fmt.Printf("Found %d table(s) in %s: %s\n", len(snap.Tables), snap.Schema, strings.Join(snap.Tables, ", "))
	fmt.Printf("Wrote %s\n", initOut)
	fmt.Printf("Next: edit any golden_query checks, then run: revenant verify -c %s\n", initOut)
	return nil
}
