package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/pawan-bisht/revenant/internal/demo"
)

var migrateCmd = &cobra.Command{
	Use:          "migrate",
	Short:        "Create the application tables in PostgreSQL",
	SilenceUsage: true,
	RunE:         runMigrate,
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

func runMigrate(cmd *cobra.Command, args []string) error {
	_ = godotenv.Load()
	connectionString := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if connectionString == "" {
		return fmt.Errorf("DATABASE_URL is not set (export it or put it in .env)")
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
