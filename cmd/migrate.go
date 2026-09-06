package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/pawan-bisht/revenant/internal/demo"
)

var migrateYes bool

var migrateCmd = &cobra.Command{
	Use:          "migrate",
	Short:        "Demo only: create sample customers/orders tables (not your app schema)",
	Long: `Creates two hardcoded demo tables (customers, orders) and seeds sample rows.

This does NOT run your application's migrations. Use only on an empty local
database when learning Revenant. Real projects should use revenant init against
tables that already exist.`,
	SilenceUsage: true,
	RunE:         runMigrate,
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.Flags().BoolVar(&migrateYes, "yes", false, "skip confirmation prompt (required in CI/non-interactive shells)")
}

func runMigrate(cmd *cobra.Command, args []string) error {
	_ = godotenv.Load()
	connectionString := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if connectionString == "" {
		return fmt.Errorf("DATABASE_URL is not set (export it or put it in .env)")
	}

	if err := confirmMigrate(); err != nil {
		return err
	}

	db, err := demo.Open(connectionString)
	if err != nil {
		return err
	}

	if err := demo.AutoMigrate(cmd.Context(), db); err != nil {
		return err
	}
	fmt.Println("customers and orders tables are ready")

	if err := demo.Seed(cmd.Context(), db); err != nil {
		return err
	}

	orders, err := demo.List(cmd.Context(), db)
	if err != nil {
		return err
	}
	for _, order := range orders {
		fmt.Printf("order %d: %s -> %s ($%.2f)\n", order.ID, order.Customer.Name, order.Product, order.Amount)
	}
	return nil
}

func confirmMigrate() error {
	if migrateYes {
		return nil
	}

	if !stdinIsInteractive() {
		return fmt.Errorf("migrate is interactive; pass --yes to confirm in scripts/CI")
	}

	fmt.Println("migrate will create TWO demo tables on DATABASE_URL and seed sample data:")
	fmt.Println("  • customers  (id, name, …)")
	fmt.Println("  • orders     (id, customer_id → customers, product, amount, …)")
	fmt.Println()
	fmt.Println("This is for learning and testing revenant.yaml only.")
	fmt.Println("It does NOT migrate your real application schema.")
	fmt.Println()
	fmt.Print("Continue? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}

	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != "yes" {
		fmt.Println("Cancelled.")
		return fmt.Errorf("migrate cancelled")
	}
	return nil
}

func stdinIsInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
