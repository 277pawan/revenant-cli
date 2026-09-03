package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/pawan-bisht/revenant/internal/checks"
	"github.com/pawan-bisht/revenant/internal/config"
	"github.com/pawan-bisht/revenant/internal/database"
	"github.com/pawan-bisht/revenant/internal/report"
)

// Flags for `revenant verify`. Cobra binds these in init() below.
var (
	configPath string // --config  path to revenant.yaml
	planName   string // --plan    optional; must match yaml "plan:" if set
	reportPath string // --output  where to write report.json
)

// verifyCmd is the Phase 1 heart of the product:
//
//  1. load .env (optional, local convenience)
//  2. parse revenant.yaml
//  3. connect to Postgres
//  4. run each check
//  5. print a human summary
//  6. write report.json
//  7. exit 1 if any check failed (so CI can use this later)
var verifyCmd = &cobra.Command{
	Use:          "verify",
	Short:        "Run restore-validation checks against a live database",
	SilenceUsage: true, // don't dump flags after a real runtime error
	RunE:         runVerify,
}

func init() {
	// Register this subcommand on the root: `revenant verify`
	rootCmd.AddCommand(verifyCmd)

	verifyCmd.Flags().StringVarP(&configPath, "config", "c", "revenant.yaml", "path to revenant.yaml")
	verifyCmd.Flags().StringVar(&planName, "plan", "", "optional plan name; must match revenant.yaml if set")
	verifyCmd.Flags().StringVarP(&reportPath, "output", "o", "report.json", "where to write the JSON report")
}

func runVerify(cmd *cobra.Command, args []string) error {
	log := slog.Default()

	// godotenv is a *local* helper. In CI you will set DATABASE_URL as a
	// secret instead. Missing .env is not an error.
	if err := godotenv.Load(); err != nil {
		log.Debug("no .env file loaded (that is fine if DATABASE_URL is already exported)", "err", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	if planName != "" && planName != cfg.Plan {
		return fmt.Errorf("--plan %q does not match yaml plan %q", planName, cfg.Plan)
	}

	log.Info("loaded config", "plan", cfg.Plan, "engine", cfg.Database.Engine, "checks", len(cfg.Checks))

	if cfg.Database.Engine != "postgres" {
		return fmt.Errorf("unsupported engine %q (phase 1 only supports postgres)", cfg.Database.Engine)
	}

	started := time.Now()

	db, err := database.Connect(cfg.Database.Connection)
	if err != nil {
		return err
	}
	defer db.Close(cmd.Context())

	log.Info("connected to postgres")

	results, err := checks.RunAll(db, cfg.Checks)
	if err != nil {
		return err
	}

	printHuman(results)

	rep := report.Build(cfg.Plan, results, time.Since(started))
	if err := report.WriteJSON(reportPath, rep); err != nil {
		return err
	}

	fmt.Printf("\nRestore Validation: %s\n", rep.Status)
	fmt.Printf("Wrote %s\n", reportPath)

	if rep.Status != report.StatusPass {
		// Returning an error makes Cobra exit non-zero.
		return fmt.Errorf("one or more checks failed")
	}
	return nil
}

// printHuman is the terminal UX for the first demo. Keep this simple;
// fancy tables / colors can wait. Check packages already put a clear
// Message on each result (e.g. "customers table exists").
func printHuman(results []checks.Result) {
	for _, r := range results {
		mark := "✗"
		if r.Status == checks.StatusPass {
			mark = "✓"
		}
		fmt.Printf("%s %s\n", mark, r.Message)
	}
}
