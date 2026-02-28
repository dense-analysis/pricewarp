// Read Cryptocurrency market data into the database
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dense-analysis/pricewarp/internal/database"
	"github.com/dense-analysis/pricewarp/internal/env"
	"github.com/shopspring/decimal"
)

var VerySmallAmount = decimal.New(1, -20)

const (
	binanceTickerURL = "https://api.binance.com/api/v3/ticker/price"
	krakenTickerURL  = "https://api.kraken.com/0/public/Ticker"
)

type BinanceTickerResult struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

type KrakenTickerResponse struct {
	Error  []string                     `json:"error"`
	Result map[string]KrakenTickerValue `json:"result"`
}

type KrakenTickerValue struct {
	Close []string `json:"c"`
}

type quoteSuffix struct {
	Suffix   string
	Currency string
}

var binanceQuoteSuffixes = []quoteSuffix{
	{Suffix: "USDT", Currency: "USD"},
	{Suffix: "USDC", Currency: "USD"},
	{Suffix: "BTC", Currency: "BTC"},
	{Suffix: "USD", Currency: "USD"},
}

var krakenUnambiguousQuoteSuffixes = []quoteSuffix{
	{Suffix: "USDT", Currency: "USD"},
	{Suffix: "USDC", Currency: "USD"},
}

var krakenStandardQuoteSuffixes = []quoteSuffix{
	{Suffix: "XBT", Currency: "BTC"},
	{Suffix: "BTC", Currency: "BTC"},
	{Suffix: "USD", Currency: "USD"},
	{Suffix: "GBP", Currency: "GBP"},
}

func readURLContent(url string) ([]byte, error) {
	response, err := http.Get(url)

	if err != nil {
		return nil, err
	}

	defer func() {
		_ = response.Body.Close()
	}()

	content, err := io.ReadAll(response.Body)

	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d: %s", response.StatusCode, strings.TrimSpace(string(content)))
	}

	return content, nil
}

func readBinanceTickerResults() ([]BinanceTickerResult, error) {
	content, err := readURLContent(binanceTickerURL)

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

func readKrakenTickerResults() (map[string]KrakenTickerValue, error) {
	content, err := readURLContent(krakenTickerURL)

	if err != nil {
		return nil, err
	}

	var payload KrakenTickerResponse

	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, fmt.Errorf("kraken api returned unexpected response: %s", string(content))
	}

	if len(payload.Error) > 0 {
		return nil, fmt.Errorf("kraken api error: %s", strings.Join(payload.Error, ", "))
	}

	if payload.Result == nil {
		return nil, fmt.Errorf("kraken api returned no results")
	}

	return payload.Result, nil
}

type CryptoPrice struct {
	From  string
	To    string
	Value string
}

type pairKey struct {
	From string
	To   string
}

type priceAggregate struct {
	Total decimal.Decimal
	Count int64
}

type currencyInfo struct {
	Name string
}

type priceInsertRow struct {
	FromTicker string
	FromName   string
	ToTicker   string
	ToName     string
	Value      decimal.Decimal
}

func parseTickerSymbol(symbol string, suffixes []quoteSuffix) (string, string, bool) {
	for _, suffix := range suffixes {
		if strings.HasSuffix(symbol, suffix.Suffix) {
			fromCurrency := symbol[:len(symbol)-len(suffix.Suffix)]

			return fromCurrency, suffix.Currency, true
		}
	}

	return "", "", false
}

func isFilteredBaseAsset(fromCurrency string) bool {
	return strings.HasSuffix(fromCurrency, "DOWN") ||
		strings.HasSuffix(fromCurrency, "UP") ||
		strings.HasSuffix(fromCurrency, "BULL") ||
		strings.HasSuffix(fromCurrency, "BEAR") ||
		(len(fromCurrency) >= 4 && strings.HasSuffix(fromCurrency, "B"))
}

func shouldSkipBinancePair(fromCurrency string, toCurrency string) bool {
	return fromCurrency == "GBP" ||
		toCurrency == "GBP" ||
		fromCurrency == "XMR" ||
		toCurrency == "XMR"
}

func normalizeKrakenCurrency(ticker string) string {
	ticker = strings.TrimSuffix(ticker, ".d")

	if len(ticker) == 4 && (strings.HasPrefix(ticker, "X") || strings.HasPrefix(ticker, "Z")) {
		ticker = ticker[1:]
	}

	switch ticker {
	case "XBT":
		return "BTC"
	case "XDG":
		return "DOGE"
	default:
		return ticker
	}
}

func isTickerAlphanumeric(ticker string) bool {
	if ticker == "" {
		return false
	}

	for _, char := range ticker {
		if (char < 'A' || char > 'Z') && (char < '0' || char > '9') {
			return false
		}
	}

	return true
}

func isKrakenPrefixedBaseTicker(baseTicker string) bool {
	if len(baseTicker) == 4 && (strings.HasPrefix(baseTicker, "X") || strings.HasPrefix(baseTicker, "Z")) {
		return true
	}

	return baseTicker == "USDT" || baseTicker == "USDC"
}

func parseKrakenTickerSymbol(symbol string) (string, string, bool) {
	normalizedSymbol := strings.TrimSuffix(symbol, ".d")

	if normalizedSymbol == "" {
		return "", "", false
	}

	fromCurrency, toCurrency, ok := parseTickerSymbol(normalizedSymbol, krakenUnambiguousQuoteSuffixes)

	if ok {
		return normalizeKrakenCurrency(fromCurrency), toCurrency, true
	}

	if strings.HasSuffix(normalizedSymbol, "XXBT") {
		prefixedBaseTicker := normalizedSymbol[:len(normalizedSymbol)-len("XXBT")]

		if isKrakenPrefixedBaseTicker(prefixedBaseTicker) {
			return normalizeKrakenCurrency(prefixedBaseTicker), "BTC", true
		}

		standardBaseTicker := normalizedSymbol[:len(normalizedSymbol)-len("XBT")]

		return normalizeKrakenCurrency(standardBaseTicker), "BTC", true
	}

	if strings.HasSuffix(normalizedSymbol, "ZUSD") {
		prefixedBaseTicker := normalizedSymbol[:len(normalizedSymbol)-len("ZUSD")]

		if isKrakenPrefixedBaseTicker(prefixedBaseTicker) || isFilteredBaseAsset(prefixedBaseTicker) {
			return normalizeKrakenCurrency(prefixedBaseTicker), "USD", true
		}

		standardBaseTicker := normalizedSymbol[:len(normalizedSymbol)-len("USD")]

		return normalizeKrakenCurrency(standardBaseTicker), "USD", true
	}

	if strings.HasSuffix(normalizedSymbol, "ZGBP") {
		prefixedBaseTicker := normalizedSymbol[:len(normalizedSymbol)-len("ZGBP")]

		if isKrakenPrefixedBaseTicker(prefixedBaseTicker) || isFilteredBaseAsset(prefixedBaseTicker) {
			return normalizeKrakenCurrency(prefixedBaseTicker), "GBP", true
		}

		standardBaseTicker := normalizedSymbol[:len(normalizedSymbol)-len("GBP")]

		return normalizeKrakenCurrency(standardBaseTicker), "GBP", true
	}

	fromCurrency, toCurrency, ok = parseTickerSymbol(normalizedSymbol, krakenStandardQuoteSuffixes)

	if ok {
		return normalizeKrakenCurrency(fromCurrency), toCurrency, true
	}

	return "", "", false
}

func readBinancePrices(results []BinanceTickerResult) []CryptoPrice {
	var prices []CryptoPrice

	for _, tickerData := range results {
		fromCurrency, toCurrency, ok := parseTickerSymbol(tickerData.Symbol, binanceQuoteSuffixes)

		if !ok || !isTickerAlphanumeric(fromCurrency) || !isTickerAlphanumeric(toCurrency) ||
			isFilteredBaseAsset(fromCurrency) || shouldSkipBinancePair(fromCurrency, toCurrency) {
			continue
		}

		prices = append(prices, CryptoPrice{
			From:  fromCurrency,
			To:    toCurrency,
			Value: tickerData.Price,
		})
	}

	return prices
}

func readKrakenPrices(results map[string]KrakenTickerValue) []CryptoPrice {
	var prices []CryptoPrice

	for symbol, tickerData := range results {
		fromCurrency, toCurrency, ok := parseKrakenTickerSymbol(symbol)

		if !ok {
			continue
		}

		if !isTickerAlphanumeric(fromCurrency) || !isTickerAlphanumeric(toCurrency) ||
			isFilteredBaseAsset(fromCurrency) || len(tickerData.Close) == 0 {
			continue
		}

		prices = append(prices, CryptoPrice{
			From:  fromCurrency,
			To:    toCurrency,
			Value: tickerData.Close[0],
		})
	}

	return prices
}

func averageByPair(prices []CryptoPrice) (map[pairKey]decimal.Decimal, error) {
	aggregates := map[pairKey]priceAggregate{}

	for _, price := range prices {
		decimalValue, err := decimal.NewFromString(price.Value)

		if err != nil {
			return nil, fmt.Errorf("invalid price for %s/%s: %w", price.From, price.To, err)
		}

		key := pairKey{From: price.From, To: price.To}
		aggregate := aggregates[key]
		aggregate.Total = aggregate.Total.Add(decimalValue)
		aggregate.Count += 1
		aggregates[key] = aggregate
	}

	priceMap := map[pairKey]decimal.Decimal{}

	for key, aggregate := range aggregates {
		priceMap[key] = aggregate.Total.Div(decimal.NewFromInt(aggregate.Count))
	}

	return priceMap, nil
}

func combinePrices(binancePrices []CryptoPrice, krakenPrices []CryptoPrice) ([]CryptoPrice, error) {
	binanceAverages, err := averageByPair(binancePrices)

	if err != nil {
		return nil, err
	}

	krakenAverages, err := averageByPair(krakenPrices)

	if err != nil {
		return nil, err
	}

	combinedTotals := map[pairKey]decimal.Decimal{}
	combinedCounts := map[pairKey]int64{}

	for key, value := range binanceAverages {
		combinedTotals[key] = combinedTotals[key].Add(value)
		combinedCounts[key] += 1
	}

	for key, value := range krakenAverages {
		combinedTotals[key] = combinedTotals[key].Add(value)
		combinedCounts[key] += 1
	}

	keys := make([]pairKey, 0, len(combinedTotals))

	for key := range combinedTotals {
		keys = append(keys, key)
	}

	sort.Slice(keys, func(i int, j int) bool {
		if keys[i].From == keys[j].From {
			return keys[i].To < keys[j].To
		}

		return keys[i].From < keys[j].From
	})

	prices := make([]CryptoPrice, 0, len(keys))

	for _, key := range keys {
		price := combinedTotals[key].Div(decimal.NewFromInt(combinedCounts[key]))

		prices = append(prices, CryptoPrice{
			From:  key.From,
			To:    key.To,
			Value: price.String(),
		})
	}

	return prices, nil
}

func loadCurrentTickerMap(conn *database.Conn) (map[string]bool, error) {
	tickerRows, err := conn.Query("SELECT ticker from crypto_currencies")

	if err != nil {
		return nil, err
	}
	defer tickerRows.Close()

	currentTickerMap := map[string]bool{}

	for tickerRows.Next() {
		var ticker string
		if err := tickerRows.Scan(&ticker); err != nil {
			return nil, err
		}
		currentTickerMap[ticker] = true
	}

	if err := tickerRows.Err(); err != nil {
		return nil, err
	}

	return currentTickerMap, nil
}

func collectMissingTickers(prices []CryptoPrice, currentTickerMap map[string]bool) []string {
	missingTickerMap := map[string]bool{}

	for _, price := range prices {
		for _, ticker := range []string{price.From, price.To} {
			if !currentTickerMap[ticker] {
				missingTickerMap[ticker] = true
			}
		}
	}

	missingTickers := make([]string, 0, len(missingTickerMap))

	for ticker := range missingTickerMap {
		missingTickers = append(missingTickers, ticker)
	}

	sort.Strings(missingTickers)

	return missingTickers
}

func writeCurrencies(conn *database.Conn, prices []CryptoPrice) error {
	currentTickerMap, err := loadCurrentTickerMap(conn)

	if err != nil {
		return err
	}

	missingTickers := collectMissingTickers(prices, currentTickerMap)

	if len(missingTickers) == 0 {
		return nil
	}

	batch, err := conn.PrepareBatch(
		`insert into crypto_currencies (ticker, name, updated_at)
		values (?, ?, ?)`,
	)

	if err != nil {
		return err
	}

	timestamp := time.Now()

	for _, ticker := range missingTickers {
		if err := batch.Append(ticker, ticker, timestamp); err != nil {
			return err
		}
	}

	return batch.Send()
}

func loadCurrencyInfoMap(conn *database.Conn) (map[string]currencyInfo, error) {
	tickerRows, err := conn.Query("SELECT ticker, name from crypto_currencies")

	if err != nil {
		return nil, err
	}
	defer tickerRows.Close()

	tickerMap := map[string]currencyInfo{}

	for tickerRows.Next() {
		var ticker string
		var name string
		if err := tickerRows.Scan(&ticker, &name); err != nil {
			return nil, err
		}
		tickerMap[ticker] = currencyInfo{Name: name}
	}

	if err := tickerRows.Err(); err != nil {
		return nil, err
	}

	return tickerMap, nil
}

func buildPriceInsertRows(prices []CryptoPrice, tickerMap map[string]currencyInfo) ([]priceInsertRow, error) {
	rows := make([]priceInsertRow, 0, len(prices))

	for _, price := range prices {
		decimalValue, decimalErr := decimal.NewFromString(price.Value)

		if decimalErr != nil {
			return nil, decimalErr
		}

		if decimalValue.LessThanOrEqual(decimal.Zero) {
			decimalValue = VerySmallAmount
		}

		fromInfo, ok := tickerMap[price.From]
		if !ok {
			return nil, fmt.Errorf("missing currency info for %s", price.From)
		}
		toInfo, ok := tickerMap[price.To]
		if !ok {
			return nil, fmt.Errorf("missing currency info for %s", price.To)
		}

		rows = append(rows, priceInsertRow{
			FromTicker: price.From,
			FromName:   fromInfo.Name,
			ToTicker:   price.To,
			ToName:     toInfo.Name,
			Value:      decimalValue,
		})
	}

	return rows, nil
}

func writePrices(conn *database.Conn, prices []CryptoPrice) error {
	timestamp := time.Now()
	tickerMap, err := loadCurrencyInfoMap(conn)

	if err != nil {
		return err
	}

	rows, err := buildPriceInsertRows(prices, tickerMap)

	if err != nil {
		return err
	}

	if len(rows) == 0 {
		return nil
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

	for _, row := range rows {
		if err := batch.Append(
			timestamp,
			row.FromTicker,
			row.FromName,
			row.ToTicker,
			row.ToName,
			row.Value,
		); err != nil {
			return err
		}
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

	binanceTickerResults, binanceErr := readBinanceTickerResults()
	var binancePrices []CryptoPrice

	if binanceErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to read Binance prices: %s\n", binanceErr)
	} else {
		binancePrices = readBinancePrices(binanceTickerResults)
	}

	krakenTickerResults, krakenErr := readKrakenTickerResults()
	var krakenPrices []CryptoPrice

	if krakenErr != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to read Kraken prices: %s\n", krakenErr)
	} else {
		krakenPrices = readKrakenPrices(krakenTickerResults)
	}

	if len(binancePrices) == 0 && len(krakenPrices) == 0 {
		fmt.Fprintf(os.Stderr, "HTTP error: no prices available from Binance or Kraken\n")
		os.Exit(1)
	}

	prices, err := combinePrices(binancePrices, krakenPrices)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Price merge error: %s\n", err)
		os.Exit(1)
	}

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
