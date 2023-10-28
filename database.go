package main

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// NewDB returns a new database handler
func NewDB(host, user, password, dbname string) (db *gorm.DB, err error) {

	if db, err = gorm.Open(postgres.Open(fmt.Sprintf(
		"host=%s user=%s dbname=%s password=%s sslmode=disable",
		host, user, dbname, password)), &gorm.Config{}); err != nil {
		return
	}

	db.AutoMigrate(
		&Entry{},
		&Config{},
	)

	return
}

type Entry struct {
	ID    string          `gorm:"not null,unique"`
	Fecha time.Time       `gorm:"not null"`
	Min   decimal.Decimal `gorm:"not null"`
	Max   decimal.Decimal `gorm:"not null"`
	Avg   decimal.Decimal `gorm:"not null"`

	CreatedAt time.Time
}

type Config struct {
	gorm.Model

	ID   uint
	Data datatypes.JSON
}

type ConfigData struct {
	MaxValue float64
}
