package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd is the command users get when they type just `revenant` with no
// subcommand. It does not run a verify itself — that lives in verify.go.
var rootCmd = &cobra.Command{
	Use:   "revenant",
	Short: "Prove that a database restore is actually usable",
	Long: `Revenant connects to a PostgreSQL database, runs the checks in
revenant.yaml, prints PASS/FAIL, and writes report.json / report.md.

  revenant init      scaffold yaml from a live database
  revenant migrate   demo only — sample customers/orders tables (prompts first)
  revenant snapshot  check source DB and create an RDS snapshot
  revenant verify   run those checks
  revenant reap     clean up lingering orphaned cloud sandboxes`,
}

// Execute is called from main.go. Cobra prints the error and we exit 1 so
// CI (GitHub Actions later) can fail the job when verify fails.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
