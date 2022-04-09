// Read Cryptocurrency market data into the database
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/dense-analysis/pricewarp/internal/database"
	"github.com/dense-analysis/pricewarp/internal/env"
)

type BinanceTickerResult struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

func readBinanceTickerResults() ([]BinanceTickerResult, error) {
	response, err := http.Get("https://api.binance.com/api/v3/ticker/price")

	if err != nil {
		return nil, err
	}

	content, err := ioutil.ReadAll(response.Body)

	if err != nil {
		return nil, err
	}

	var results []BinanceTickerResult

	if err := json.Unmarshal(content, &results); err != nil {
		return nil, err
	}

	return results, nil
}

type CryptoPrice struct {
	From  string
	To    string
	Value string
}

var suffixes = []string{
	"BTC",
	"USD",
	"USDT",
	"USDC",
	"GBP",
}

func readPrices(results []BinanceTickerResult) []CryptoPrice {
	var prices []CryptoPrice

	for _, tickerData := range results {
		for _, suffix := range suffixes {
			if strings.HasSuffix(tickerData.Symbol, suffix) {
				realCurrency := suffix

				if suffix == "USDT" || suffix == "USDC" {
					realCurrency = "USD"
				}

				fromCurrency := tickerData.Symbol[:len(tickerData.Symbol)-len(suffix)]

				if !strings.HasSuffix(fromCurrency, "DOWN") &&
					!strings.HasSuffix(fromCurrency, "UP") &&
					!strings.HasSuffix(fromCurrency, "BULL") &&
					!strings.HasSuffix(fromCurrency, "BEAR") &&
					(len(fromCurrency) < 4 || !strings.HasSuffix(fromCurrency, "B")) {
					prices = append(prices, CryptoPrice{
						fromCurrency,
						realCurrency,
						tickerData.Price,
					})
				}
			}
		}
	}

	return prices
}

func writeCurrencies(transaction *database.Tx, prices []CryptoPrice) error {
	tickerRows, err := transaction.Query("SELECT ticker from crypto_currency")

	if err != nil {
		return err
	}

	currentTickerMap := map[string]bool{}

	for tickerRows.Next() {
		var ticker string
		tickerRows.Scan(&ticker)
		currentTickerMap[ticker] = true
	}

	var inputRows [][]interface{}

	for _, price := range prices {
		for _, ticker := range []string{price.From, price.To} {
			if !currentTickerMap[ticker] {
				inputRows = append(inputRows, []interface{}{ticker, ticker})
				currentTickerMap[ticker] = true
			}
		}
	}

	if len(inputRows) > 0 {
		_, err = transaction.CopyFrom("crypto_currency", []string{"ticker", "name"}, inputRows)
	}

	return err
}

func writePrices(transaction *database.Tx, prices []CryptoPrice) error {
	timestamp := time.Now()
	tickerRows, err := transaction.Query("SELECT id, ticker from crypto_currency")

	if err != nil {
		return err
	}

	tickerMap := map[string]int{}

	for tickerRows.Next() {
		var id int
		var ticker string
		tickerRows.Scan(&id, &ticker)
		tickerMap[ticker] = id
	}

	var inputRows [][]interface{}

	for _, price := range prices {
		decimalValue, decimalErr := decimal.NewFromString(price.Value)

		if decimalErr != nil {
			return decimalErr
		}

		inputRows = append(inputRows, []interface{}{
			tickerMap[price.From],
			tickerMap[price.To],
			timestamp,
			decimalValue,
		})
	}

	if len(inputRows) > 0 {
		_, err = transaction.CopyFrom("crypto_price", []string{"from", "to", "time", "value"}, inputRows)
	}

	return err
}

func main() {
	env.LoadEnvironmentVariables()

	conn, err := database.Connect()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Connection error: %s\n", err)
		os.Exit(1)
	}

	defer conn.Close()

	tickerResults, err := readBinanceTickerResults()

	if err != nil {
		fmt.Fprintf(os.Stderr, "HTTP error: %s\n", err)
		os.Exit(1)
	}

	prices := readPrices(tickerResults)

	transaction, err := conn.Begin()

	if err != nil {
		fmt.Fprintf(os.Stderr, "SQL error: %s\n", err)
		os.Exit(1)
	}

	defer transaction.Rollback()

	err = writeCurrencies(transaction, prices)

	if err != nil {
		fmt.Fprintf(os.Stderr, "SQL error: %s\n", err)
		os.Exit(1)
	}

	err = writePrices(transaction, prices)

	if err != nil {
		fmt.Fprintf(os.Stderr, "SQL error: %s\n", err)
		os.Exit(1)
	}

	transaction.Commit()
}
