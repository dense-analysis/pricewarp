package model

import (
	"time"
	"github.com/shopspring/decimal"
)

// User represents a user in the database
type User struct {
	ID int
	Username string
}

// Currency represents a currency in the database
type Currency struct {
	ID int
	Ticker string
	Name string
}

// Alert represents an alert configured by a user
type Alert struct {
	ID int
	From Currency
	To Currency
	Value decimal.Decimal
	Above bool
	Time time.Time
	Sent bool
}

// Portfolio represents portfolio data for a user
type Portfolio struct {
	Currency Currency
	Cash decimal.Decimal
}

// Asset represents the value and purchased amount of a crypto asset
type Asset struct {
	Currency Currency
	Purchased decimal.Decimal
	Amount decimal.Decimal
}
