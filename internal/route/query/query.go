package query

import (
	"sort"

	"github.com/dense-analysis/pricewarp/internal/database"
	"github.com/dense-analysis/pricewarp/internal/model"
)

var currencyQuery = `select id, ticker, name from crypto_currency `

func scanCurrency(row database.Row, currency *model.Currency) error {
	return row.Scan(&currency.ID, &currency.Ticker, &currency.Name)
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

// LoadCurrencyByID loads a single by ID.
func LoadCurrencyByID(conn *database.Conn, currency *model.Currency, currencyID int) error {
	row := conn.QueryRow(currencyQuery+"where id = $1", currencyID)

	return scanCurrency(row, currency)
}

func indexOfString(array []string, element string) int {
	for i, v := range array {
		if element == v {
			return i
		}
	}

	return -1
}

var toCurrencyTickers = []string{
	"USD",
	"GBP",
	"BTC",
}

type byTickerOrder []model.Currency

func (a byTickerOrder) Len() int {
	return len(a)
}

func (a byTickerOrder) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a byTickerOrder) Less(i, j int) bool {
	leftIndex := indexOfString(toCurrencyTickers, a[i].Ticker)
	rightIndex := indexOfString(toCurrencyTickers, a[j].Ticker)

	return leftIndex < rightIndex
}

func isToCurrency(currency *model.Currency) bool {
	for _, ticker := range toCurrencyTickers {
		if currency.Ticker == ticker {
			return true
		}
	}

	return false
}

// BuildToCurrencyList creates a new list of only the availalbe "to" currencies for a portfolio
func BuildToCurrencyList(currencyList []model.Currency) []model.Currency {
	fiatCurrencyList := make([]model.Currency, 0, len(toCurrencyTickers))

	for _, currency := range currencyList {
		if isToCurrency(&currency) {
			fiatCurrencyList = append(fiatCurrencyList, currency)
		}
	}

	sort.Sort(byTickerOrder(fiatCurrencyList))

	return fiatCurrencyList
}
