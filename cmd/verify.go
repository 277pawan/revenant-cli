package cmd

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/pawan-bisht/revenant/internal/checks"
	"github.com/pawan-bisht/revenant/internal/config"
	"github.com/pawan-bisht/revenant/internal/database"
	"github.com/pawan-bisht/revenant/internal/recovery"
	"github.com/pawan-bisht/revenant/internal/report"
)

// Flags for `revenant verify`. Cobra binds these in init() below.
var (
	configPath           string // --config   path to revenant.yaml
	planName             string // --plan     optional; must match yaml "plan:" if set
	reportPath           string // --output   where to write report.json
	markdownPath         string // --markdown where to write report.md
	keepSandboxOnFailure bool   // --keep-sandbox-on-failure preserve AWS sandbox for debugging
)

// verifyCmd is the Phase 1-4 heart of the product:
//
//  1. load .env (optional, local convenience)
//  2. parse revenant.yaml
//     3a. [Phase 1-2] connect directly to Postgres, OR
//     3b. [Phase 3-4] if recovery.engine=aws-rds: restore snapshot, wait, connect
//  4. run each check
//  5. print a human summary
//  6. write report.json and report.md
//  7. cleanup (destroy sandbox if AWS)
//  8. exit 1 if any check failed (so CI can use this later)
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
	verifyCmd.Flags().StringVar(&markdownPath, "markdown", "report.md", "where to write the Markdown report")
	verifyCmd.Flags().BoolVar(&keepSandboxOnFailure, "keep-sandbox-on-failure", false, "preserve the AWS sandbox when validation fails")
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
	var sandboxID string // track for cleanup if needed
	var db *database.Connection
	var cleanupErr error
	cleanupSandbox := true

	// Decide: restore from AWS snapshot or connect directly?
	if cfg.Recovery.Engine == "aws-rds" {
		log.Info("AWS recovery mode enabled", "source", cfg.Recovery.SourceIdentifier)
		var restoreErr error
		sandboxID, db, restoreErr = restoreFromAWSSnapshot(cmd, cfg, log)
		defer func() {
			if sandboxID != "" && cleanupSandbox {
				log.Info("cleaning up sandbox", "id", sandboxID)
				cleanupErr = destroyAWSSnapshot(cmd, cfg, sandboxID, log)
			} else if sandboxID != "" {
				log.Warn("keeping sandbox for debugging", "id", sandboxID)
			}
		}()
		if restoreErr != nil {
			cleanupSandbox = !keepSandboxOnFailure
			return restoreErr
		}
		if db == nil {
			return fmt.Errorf("AWS restore returned no database connection")
		}
	} else {
		// Direct connection mode (phase 1-2)
		log.Info("direct connection mode (no AWS restore)")
		var connErr error
		db, connErr = database.ConnectURL(cfg.Database.Connection)
		if connErr != nil {
			return connErr
		}
	}

	defer db.Close(cmd.Context())

	log.Info("connected to postgres")

	results, err := checks.RunAll(db.Conn(), cfg.Checks)
	if err != nil {
		return err
	}

	printHuman(results)

	rep := report.Build(cfg.Plan, results, time.Since(started))
	if err := report.WriteJSON(reportPath, rep); err != nil {
		return err
	}
	if err := report.WriteMarkdown(markdownPath, rep); err != nil {
		return err
	}

	fmt.Printf("\nRestore Validation: %s\n", rep.Status)
	fmt.Printf("Recovery Time (RTO): %s\n", rep.Duration)
	fmt.Printf("Wrote %s\n", reportPath)
	fmt.Printf("Wrote %s\n", markdownPath)

	if cleanupErr != nil {
		log.Warn("sandbox cleanup failed", "err", cleanupErr)
	}

	if rep.Status != report.StatusPass {
		cleanupSandbox = !keepSandboxOnFailure
		// Returning an error makes Cobra exit non-zero.
		return fmt.Errorf("one or more checks failed")
	}
	return nil
}

// restoreFromAWSSnapshot handles the full AWS workflow:
// 1. Find latest snapshot
// 2. Restore to sandbox
// 3. Wait for availability
// 4. Extract endpoint
// 5. Connect with credentials
func restoreFromAWSSnapshot(cmd *cobra.Command, cfg *config.File, log *slog.Logger) (string, *database.Connection, error) {
	ctx := cmd.Context()

	// Create AWS client
	awsClient, err := recovery.NewAWSClient(ctx, cfg.Recovery.Region)
	if err != nil {
		return "", nil, fmt.Errorf("connect to AWS: %w", err)
	}

	// Find latest snapshot
	log.Info("finding latest snapshot", "source", cfg.Recovery.SourceIdentifier)
	snapshot, err := awsClient.FindLatestSnapshot(ctx, cfg.Recovery.SourceIdentifier)
	if err != nil {
		return "", nil, fmt.Errorf("find snapshot: %w", err)
	}
	log.Info("found snapshot", "snapshot_arn", *snapshot.DBSnapshotIdentifier, "created", snapshot.SnapshotCreateTime)

	// Generate unique sandbox ID (plan-name-randomsuffix)
	sandboxID := generateSandboxID(cfg.Plan)
	log.Info("restoring to sandbox", "sandbox_id", sandboxID)

	// Determine instance class (use free tier if requested)
	instanceClass := recoveryInstanceClass(cfg.Recovery.SandboxInstanceClass, cfg.Recovery.UseFreetier)
	if cfg.Recovery.UseFreetier && cfg.Recovery.SandboxInstanceClass == "" {
		log.Info("using AWS free tier compatible instance (db.t3.micro)")
	}

	// Restore snapshot to sandbox
	restoreOpts := recovery.RestoreOptions{
		SnapshotARN:   *snapshot.DBSnapshotIdentifier,
		SandboxDBID:   sandboxID,
		InstanceClass: instanceClass,
		PlanName:      cfg.Plan,
	}

	if err := awsClient.RestoreSandbox(ctx, restoreOpts); err != nil {
		return sandboxID, nil, fmt.Errorf("restore snapshot: %w", err)
	}

	// Wait for sandbox to be available (default 30 min timeout)
	log.Info("waiting for sandbox to become available")
	if err := awsClient.WaitForDB(ctx, sandboxID, 30*time.Second, 30*time.Minute); err != nil {
		return sandboxID, nil, fmt.Errorf("wait for DB: %w", err)
	}

	// Extract endpoint
	endpoint, err := awsClient.GetEndpoint(ctx, sandboxID)
	if err != nil {
		return sandboxID, nil, fmt.Errorf("get endpoint: %w", err)
	}

	// Build connection string: postgres://user:pass@host:port/dbname?sslmode=...
	// The dbname from the snapshot is preserved, we need master user creds
	connStr, err := buildConnectionString(endpoint, cfg.Database.Connection)
	if err != nil {
		return sandboxID, nil, fmt.Errorf("build sandbox connection string: %w", err)
	}
	log.Info("connecting to sandbox database", "endpoint", endpoint)

	// Connect to sandbox
	conn, err := database.ConnectURL(connStr)
	if err != nil {
		return sandboxID, nil, fmt.Errorf("connect to sandbox: %w", err)
	}

	return sandboxID, conn, nil
}

func recoveryInstanceClass(configuredClass string, useFreetier bool) string {
	if configuredClass != "" {
		return configuredClass
	}
	if useFreetier {
		return "db.t3.micro"
	}
	return "db.t4g.micro"
}

// destroyAWSSnapshot destroys the temporary sandbox instance
func destroyAWSSnapshot(cmd *cobra.Command, cfg *config.File, sandboxID string, log *slog.Logger) error {
	ctx := cmd.Context()

	awsClient, err := recovery.NewAWSClient(ctx, cfg.Recovery.Region)
	if err != nil {
		return fmt.Errorf("connect to AWS for cleanup: %w", err)
	}

	log.Info("destroying sandbox", "sandbox_id", sandboxID)
	return awsClient.DestroySandbox(ctx, sandboxID)
}

// generateSandboxID creates a unique name for the temporary DB instance
func generateSandboxID(planName string) string {
	// Format: plan-name-XXXXXX (random hex to avoid collisions)
	randomBytes := make([]byte, 3)
	rand.Read(randomBytes)
	randomSuffix := fmt.Sprintf("%x", randomBytes)
	return fmt.Sprintf("%s-%s", planName, randomSuffix)
}

// buildConnectionString modifies the connection URL to point to the sandbox endpoint
// Example: original="postgres://master:pass@prod.rds.amazonaws.com/db"
//
//	endpoint="sandbox.rds.amazonaws.com:5432"
//	result="postgres://master:pass@sandbox.rds.amazonaws.com:5432/db"
func buildConnectionString(endpoint string, originalConnStr string) (string, error) {
	originalConnStr = strings.Replace(originalConnStr, "${SANDBOX_ENDPOINT}", "sandbox.invalid", 1)
	parsed, err := url.Parse(originalConnStr)
	if err != nil {
		return "", fmt.Errorf("parse database URL: %w", err)
	}
	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return "", fmt.Errorf("unsupported database URL scheme %q", parsed.Scheme)
	}

	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimPrefix(endpoint, "postgres://")
	endpoint = strings.TrimPrefix(endpoint, "postgresql://")
	endpoint = strings.TrimSuffix(endpoint, "/")
	if endpoint == "" {
		return "", fmt.Errorf("AWS returned an empty database endpoint")
	}
	parsed.Host = endpoint
	return parsed.String(), nil
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
