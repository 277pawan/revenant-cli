package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/pawan-bisht/revenant/internal/config"
	"github.com/pawan-bisht/revenant/internal/database"
)

var doctorConfig string

var doctorCmd = &cobra.Command{
	Use:          "doctor",
	Short:        "Check environment, config file, and database connectivity",
	SilenceUsage: true,
	RunE:         runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().StringVarP(&doctorConfig, "config", "c", "revenant.yaml", "path to revenant.yaml")
}

func runDoctor(cmd *cobra.Command, args []string) error {
	_ = godotenv.Load()

	fmt.Println("Revenant doctor")
	fmt.Println()

	ok := true
	warn := func(msg string) {
		fmt.Printf("⚠ %s\n", msg)
	}
	pass := func(msg string) {
		fmt.Printf("✓ %s\n", msg)
	}
	fail := func(msg string) {
		fmt.Printf("✗ %s\n", msg)
		ok = false
	}

	cfg, err := config.Load(doctorConfig)
	if err != nil {
		fail(fmt.Sprintf("config %s: %v", doctorConfig, err))
	} else {
		pass(fmt.Sprintf("config %s loads (%d checks, plan=%q)", doctorConfig, len(cfg.Checks), cfg.Plan))
		if err := config.ValidateChecks(cfg.Checks); err != nil {
			fail(err.Error())
		} else {
			pass("all checks have valid type and required fields")
		}
	}

	if cfg != nil && cfg.Recovery.Engine == "aws-rds" {
		for _, key := range []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"} {
			if os.Getenv(key) == "" {
				warn(key + " not set (needed for AWS restore)")
			} else {
				pass(key + " is set")
			}
		}
		for _, key := range []string{"SANDBOX_USER", "SANDBOX_PASSWORD", "SANDBOX_DBNAME"} {
			if os.Getenv(key) == "" {
				warn(key + " not set (needed for AWS sandbox login)")
			}
		}
	}

	connStr := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if connStr == "" && cfg != nil && cfg.Recovery.Engine != "aws-rds" {
		fail("DATABASE_URL is not set")
	} else if connStr != "" {
		db, err := database.ConnectURL(connStr)
		if err != nil {
			fail(fmt.Sprintf("DATABASE_URL connect: %v", err))
		} else {
			defer db.Close(cmd.Context())
			pass("DATABASE_URL connects and pings OK")
		}
	} else if cfg != nil && cfg.Recovery.Engine == "aws-rds" {
		warn("DATABASE_URL not set — OK for verify (sandbox endpoint discovered at runtime); required for snapshot")
	}

	fmt.Println()
	if ok {
		fmt.Println("Doctor: PASS — ready for revenant verify")
		return nil
	}
	fmt.Println("Doctor: FAIL — fix items above before revenant verify")
	return fmt.Errorf("doctor found problems")
}
