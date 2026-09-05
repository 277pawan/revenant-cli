// Package demo contains temporary example tables used to exercise Revenant
// against a real PostgreSQL database. It can be removed without affecting the
// core restore-validation workflow.
package demo

import "time"

type Customer struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"not null"`
	Email     string `gorm:"uniqueIndex;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Orders    []Order `gorm:"foreignKey:CustomerID"`
}

type Order struct {
	ID         uint     `gorm:"primaryKey"`
	CustomerID uint     `gorm:"not null;index"`
	Customer   Customer `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	Product    string   `gorm:"not null"`
	Amount     float64  `gorm:"not null"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
