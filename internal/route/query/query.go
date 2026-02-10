package query

import (
	"slices"

	"github.com/dense-analysis/pricewarp/internal/database"
	"github.com/dense-analysis/pricewarp/internal/model"
)

var currencyQuery = `select ticker, name from crypto_currencies `

func scanCurrency(row database.Row, currency *model.Currency) error {
	return row.Scan(&currency.Ticker, &currency.Name)
}

// LoadCurrencyList loads all available currencyies into a list.
func LoadCurrencyList(conn *database.Conn, currencyList *[]model.Currency) error {
	return model.LoadList(
		conn,
		currencyList,
		500,
		scanCurrency,
		currencyQuery+"order by name",
	)
}

// LoadCurrencyByTicker loads a single by ticker.
func LoadCurrencyByTicker(conn *database.Conn, currency *model.Currency, ticker string) error {
	row := conn.QueryRow(currencyQuery+"where ticker = ?", ticker)

	return scanCurrency(row, currency)
}

var toCurrencies = []model.Currency{
	{Ticker: "USD", Name: "USD"},
	{Ticker: "GBP", Name: "GBP"},
	{Ticker: "BTC", Name: "BTC"},
}

// GetToCurrencyList returns all of the currencies that can be used as a basis of conversion for a portfolio.
// The list is computed at runtime and requires no database access.
func GetToCurrencyList() []model.Currency {
	return slices.Clone(toCurrencies)
}
