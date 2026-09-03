package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd is the command users get when they type just `revenant` with no
// subcommand. It does not run a verify itself — that lives in verify.go.
//
// NEXT: add more subcommands with rootCmd.AddCommand(...) inside init(),
// the same way verify.go already does. Examples you will want later:
//   - revenant init     (auto-generate a starter revenant.yaml from the DB)
//   - revenant report   (pretty-print an existing report.json)
var rootCmd = &cobra.Command{
	Use:   "revenant",
	Short: "Prove that a database restore is actually usable",
	Long: `Revenant connects to a PostgreSQL database, runs the checks in
revenant.yaml, prints PASS/FAIL, and writes report.json.

Phase 1 only talks to a live database (local Postgres). Snapshot restore
and AWS RDS come later — the check + report engines stay the same.`,
}

// Execute is called from main.go. Cobra prints the error and we exit 1 so
// CI (GitHub Actions later) can fail the job when verify fails.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
