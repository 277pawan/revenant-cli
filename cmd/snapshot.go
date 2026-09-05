package cmd

import (
	"fmt"
	"os"
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

var snapshotCmd = &cobra.Command{
	Use:          "snapshot",
	Short:        "Check the source database and create an RDS snapshot",
	SilenceUsage: true,
	RunE:         runSnapshot,
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
	snapshotCmd.Flags().StringVarP(&configPath, "config", "c", "revenant.yaml", "path to revenant.yaml")
	snapshotCmd.Flags().StringVarP(&reportPath, "output", "o", "report.json", "where to write the JSON report")
	snapshotCmd.Flags().StringVar(&markdownPath, "markdown", "report.md", "where to write the Markdown report")
}

func runSnapshot(cmd *cobra.Command, args []string) error {
	if err := godotenv.Load(); err != nil {
		// .env is optional when variables are exported by the shell or CI.
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	if cfg.Recovery.Engine != "aws-rds" {
		return fmt.Errorf("snapshot requires recovery.engine: aws-rds")
	}
	if cfg.Recovery.SourceIdentifier == "" {
		return fmt.Errorf("recovery.source_identifier is required")
	}

	sourceURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if sourceURL == "" {
		return fmt.Errorf("DATABASE_URL must point to the source RDS database")
	}

	started := time.Now()
	db, err := database.ConnectURL(sourceURL)
	if err != nil {
		return fmt.Errorf("connect to source database: %w", err)
	}
	defer db.Close(cmd.Context())

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
	if rep.Status != report.StatusPass {
		return fmt.Errorf("source database checks failed; snapshot was not created")
	}

	awsClient, err := recovery.NewAWSClient(cmd.Context(), cfg.Recovery.Region)
	if err != nil {
		return fmt.Errorf("connect to AWS: %w", err)
	}
	snapshotID := fmt.Sprintf("%s-%d", cfg.Plan, time.Now().UTC().Unix())
	if err := awsClient.CreateSnapshot(cmd.Context(), cfg.Recovery.SourceIdentifier, snapshotID); err != nil {
		return err
	}
	if err := awsClient.WaitForSnapshot(cmd.Context(), snapshotID, 30*time.Second, 30*time.Minute); err != nil {
		return err
	}

	fmt.Printf("Created available RDS snapshot %s from %s\n", snapshotID, cfg.Recovery.SourceIdentifier)
	fmt.Printf("Wrote %s\n", reportPath)
	fmt.Printf("Wrote %s\n", markdownPath)
	return nil
}
