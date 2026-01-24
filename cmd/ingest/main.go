// Read Cryptocurrency market data into the database
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dense-analysis/pricewarp/internal/database"
	"github.com/dense-analysis/pricewarp/internal/env"
	"github.com/shopspring/decimal"
)

var VerySmallAmount = decimal.New(1, -20)

type BinanceTickerResult struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

func readBinanceTickerResults() ([]BinanceTickerResult, error) {
	response, err := http.Get("https://api.binance.com/api/v3/ticker/price")

	if err != nil {
		return nil, err
	}

	content, err := io.ReadAll(response.Body)

	if err != nil {
		return nil, err
	}

	var results []BinanceTickerResult

	if err := json.Unmarshal(content, &results); err == nil {
		return results, nil
	}

	var apiError struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}

	if err := json.Unmarshal(content, &apiError); err == nil && apiError.Msg != "" {
		return nil, fmt.Errorf("binance api error: %d %s", apiError.Code, apiError.Msg)
	}

	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	var payload map[string]any

	if err := decoder.Decode(&payload); err == nil {
		return nil, fmt.Errorf("binance api returned unexpected payload: %v", payload)
	}

	return nil, fmt.Errorf("binance api returned unexpected response: %s", string(content))
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

				if suffix == "USDT" {
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

func writeCurrencies(conn *database.Conn, prices []CryptoPrice) error {
	tickerRows, err := conn.Query("SELECT ticker from crypto_currencies")

	if err != nil {
		return err
	}
	defer tickerRows.Close()

	currentTickerMap := map[string]bool{}

	for tickerRows.Next() {
		var ticker string
		if err := tickerRows.Scan(&ticker); err != nil {
			return err
		}
		currentTickerMap[ticker] = true
	}

	if err := tickerRows.Err(); err != nil {
		return err
	}

	batch, err := conn.PrepareBatch(
		`insert into crypto_currencies (ticker, name, updated_at)
		values (?, ?, now64(9))`,
	)

	if err != nil {
		return err
	}

	rowCount := 0

	for _, price := range prices {
		for _, ticker := range []string{price.From, price.To} {
			if !currentTickerMap[ticker] {
				if err := batch.Append(ticker, ticker); err != nil {
					return err
				}

				rowCount += 1
				currentTickerMap[ticker] = true
			}
		}
	}

	if rowCount == 0 {
		return nil
	}

	return batch.Send()
}

func writePrices(conn *database.Conn, prices []CryptoPrice) error {
	timestamp := time.Now()
	tickerRows, err := conn.Query("SELECT ticker, name from crypto_currencies")

	if err != nil {
		return err
	}
	defer tickerRows.Close()

	type currencyInfo struct {
		Name string
	}
	tickerMap := map[string]currencyInfo{}

	for tickerRows.Next() {
		var ticker string
		var name string
		if err := tickerRows.Scan(&ticker, &name); err != nil {
			return err
		}
		tickerMap[ticker] = currencyInfo{Name: name}
	}

	if err := tickerRows.Err(); err != nil {
		return err
	}

	batch, err := conn.PrepareBatch(
		`insert into crypto_currency_prices
			(time, from_currency_ticker, from_currency_name,
			 to_currency_ticker, to_currency_name, value)
		values (?, ?, ?, ?, ?, ?)`,
	)

	if err != nil {
		return err
	}

	rowCount := 0

	for _, price := range prices {
		decimalValue, decimalErr := decimal.NewFromString(price.Value)

		if decimalErr != nil {
			return decimalErr
		}

		// Hack a very small amount for 0 or negative prices.
		if decimalValue.LessThanOrEqual(decimal.Zero) {
			decimalValue = VerySmallAmount
		}

		fromInfo, ok := tickerMap[price.From]
		if !ok {
			return fmt.Errorf("missing currency info for %s", price.From)
		}
		toInfo, ok := tickerMap[price.To]
		if !ok {
			return fmt.Errorf("missing currency info for %s", price.To)
		}

		if err := batch.Append(
			timestamp,
			price.From,
			fromInfo.Name,
			price.To,
			toInfo.Name,
			decimalValue,
		); err != nil {
			return err
		}

		rowCount += 1
	}

	if rowCount == 0 {
		return nil
	}

	return batch.Send()
}

func main() {
	env.LoadEnvironmentVariables()

	conn, err := database.Connect()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Connection error: %s\n", err)
		os.Exit(1)
	}

	defer func() {
		_ = conn.Close()
	}()

	tickerResults, err := readBinanceTickerResults()

	if err != nil {
		fmt.Fprintf(os.Stderr, "HTTP error: %s\n", err)
		os.Exit(1)
	}

	prices := readPrices(tickerResults)

	err = writeCurrencies(conn, prices)

	if err != nil {
		fmt.Fprintf(os.Stderr, "SQL error: %s\n", err)
		os.Exit(1)
	}

	err = writePrices(conn, prices)

	if err != nil {
		fmt.Fprintf(os.Stderr, "SQL error: %s\n", err)
		os.Exit(1)
	}
}
