package demo

import (
	"context"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Open(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open demo database: %w", err)
	}
	return db, nil
}

func AutoMigrate(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).AutoMigrate(&Customer{}, &Order{}); err != nil {
		return fmt.Errorf("auto-migrate demo tables: %w", err)
	}
	return nil
}

func Seed(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		customers := []Customer{
			{Name: "Alice", Email: "alice@example.com"},
			{Name: "Bob", Email: "bob@example.com"},
		}
		for _, customer := range customers {
			if err := tx.Where(Customer{Email: customer.Email}).FirstOrCreate(&customer).Error; err != nil {
				return fmt.Errorf("seed customer %s: %w", customer.Email, err)
			}
		}

		var orderCount int64
		if err := tx.Model(&Order{}).Count(&orderCount).Error; err != nil {
			return fmt.Errorf("count demo orders: %w", err)
		}
		if orderCount != 0 {
			return nil
		}

		var alice, bob Customer
		if err := tx.Where("email = ?", "alice@example.com").First(&alice).Error; err != nil {
			return fmt.Errorf("find Alice: %w", err)
		}
		if err := tx.Where("email = ?", "bob@example.com").First(&bob).Error; err != nil {
			return fmt.Errorf("find Bob: %w", err)
		}

		orders := []Order{
			{CustomerID: alice.ID, Product: "Backup validation", Amount: 49.99},
			{CustomerID: bob.ID, Product: "Recovery report", Amount: 19.99},
		}
		if err := tx.Create(&orders).Error; err != nil {
			return fmt.Errorf("seed orders: %w", err)
		}
		return nil
	})
}

func List(ctx context.Context, db *gorm.DB) ([]Order, error) {
	var orders []Order
	if err := db.WithContext(ctx).Preload("Customer").Order("id").Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("list demo orders: %w", err)
	}
	return orders, nil
}
