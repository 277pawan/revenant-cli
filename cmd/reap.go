package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/pawan-bisht/revenant/internal/recovery"
)

var (
	reapMaxAge string
	reapRegion string
)

var reapCmd = &cobra.Command{
	Use:          "reap",
	Short:        "Destroy lingering orphaned Revenant sandboxes in cloud provider (AWS RDS)",
	SilenceUsage: true,
	RunE:         runReap,
}

func init() {
	rootCmd.AddCommand(reapCmd)

	reapCmd.Flags().StringVar(&reapMaxAge, "max-age", "2h", "threshold age after which sandboxes are considered orphaned (e.g., 2h, 30m)")
	reapCmd.Flags().StringVar(&reapRegion, "region", "", "AWS region (overrides default AWS_REGION environment variable)")
}

func runReap(cmd *cobra.Command, args []string) error {
	_ = godotenv.Load()

	maxAgeDuration, err := time.ParseDuration(reapMaxAge)
	if err != nil {
		return fmt.Errorf("invalid --max-age duration %q: %w", reapMaxAge, err)
	}

	ctx := cmd.Context()
	awsClient, err := recovery.NewAWSClient(ctx, reapRegion)
	if err != nil {
		return fmt.Errorf("connect to AWS RDS: %w", err)
	}

	slog.Info("scanning for orphaned sandboxes", "max_age", reapMaxAge)
	summary, err := awsClient.ReapOrphans(ctx, maxAgeDuration)
	if err != nil {
		return err
	}

	fmt.Printf("\n--- Orphan Reaper Summary ---\n")
	fmt.Printf("Instances Inspected: %d\n", summary.InspectedCount)
	fmt.Printf("Orphans Found:       %d\n", summary.OrphansFound)
	fmt.Printf("Reaped Sandboxes:    %d\n", len(summary.ReapedIDs))

	for _, id := range summary.ReapedIDs {
		fmt.Printf("  ✓ Reaped %s\n", id)
	}
	for _, errStr := range summary.Errors {
		fmt.Printf("  ✗ Error: %s\n", errStr)
	}

	if len(summary.Errors) > 0 {
		return fmt.Errorf("reaper encountered %d error(s) during cleanup", len(summary.Errors))
	}

	return nil
}
