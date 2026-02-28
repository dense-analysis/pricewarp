package main

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func sortCryptoPrices(prices []CryptoPrice) {
	sort.Slice(prices, func(i int, j int) bool {
		if prices[i].From == prices[j].From {
			if prices[i].To == prices[j].To {
				return prices[i].Value < prices[j].Value
			}

			return prices[i].To < prices[j].To
		}

		return prices[i].From < prices[j].From
	})
}

func TestReadBinancePricesFiltersAndNormalizes(t *testing.T) {
	results := []BinanceTickerResult{
		{Symbol: "BTCUSDT", Price: "100"},
		{Symbol: "AAVEBTC", Price: "0.010"},
		{Symbol: "GBPBTC", Price: "0.00003"},
		{Symbol: "XMRBTC", Price: "0.005"},
		{Symbol: "ETHDOWNUSDT", Price: "1"},
		{Symbol: "DOGEUSDC", Price: "0.12"},
		{Symbol: "BADPAIR", Price: "9"},
	}

	got := readBinancePrices(results)
	want := []CryptoPrice{
		{From: "BTC", To: "USD", Value: "100"},
		{From: "AAVE", To: "BTC", Value: "0.010"},
		{From: "DOGE", To: "USD", Value: "0.12"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected Binance prices\nwant: %#v\n got: %#v", want, got)
	}
}

func TestReadKrakenPricesParsesAndNormalizes(t *testing.T) {
	results := map[string]KrakenTickerValue{
		"XXBTZUSD": {Close: []string{"43000"}},
		"XXMRZUSD": {Close: []string{"150"}},
		"AAVEGBP":  {Close: []string{"85"}},
		"XDGUSD.d": {Close: []string{"0.1"}},
		"USD_CREDITZUSD": {
			Close: []string{"1"},
		},
		"ETHUPZUSD": {
			Close: []string{"1"},
		},
		"ADAZUSD": {Close: []string{}},
	}

	got := readKrakenPrices(results)
	sortCryptoPrices(got)

	want := []CryptoPrice{
		{From: "AAVE", To: "GBP", Value: "85"},
		{From: "BTC", To: "USD", Value: "43000"},
		{From: "DOGE", To: "USD", Value: "0.1"},
		{From: "XMR", To: "USD", Value: "150"},
	}
	sortCryptoPrices(want)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected Kraken prices\nwant: %#v\n got: %#v", want, got)
	}
}

func TestParseKrakenTickerSymbolHandlesAmbiguousPairs(t *testing.T) {
	testCases := []struct {
		Symbol string
		From   string
		To     string
		OK     bool
	}{
		{Symbol: "BLZUSD", From: "BLZ", To: "USD", OK: true},
		{Symbol: "CHZUSD", From: "CHZ", To: "USD", OK: true},
		{Symbol: "XTZUSD", From: "XTZ", To: "USD", OK: true},
		{Symbol: "TRXXBT", From: "TRX", To: "BTC", OK: true},
		{Symbol: "SNXXBT", From: "SNX", To: "BTC", OK: true},
		{Symbol: "XXMRXXBT", From: "XMR", To: "BTC", OK: true},
		{Symbol: "XXMRZUSD", From: "XMR", To: "USD", OK: true},
		{Symbol: "XXBTZUSD", From: "BTC", To: "USD", OK: true},
		{Symbol: "XETHZGBP", From: "ETH", To: "GBP", OK: true},
		{Symbol: "USDTZUSD", From: "USDT", To: "USD", OK: true},
		{Symbol: "AI16ZUSD", From: "AI16Z", To: "USD", OK: true},
		{Symbol: "NOTAPAIR", From: "", To: "", OK: false},
	}

	for _, testCase := range testCases {
		fromCurrency, toCurrency, ok := parseKrakenTickerSymbol(testCase.Symbol)

		if ok != testCase.OK {
			t.Fatalf("symbol %q expected ok=%v, got ok=%v", testCase.Symbol, testCase.OK, ok)
		}

		if fromCurrency != testCase.From || toCurrency != testCase.To {
			t.Fatalf(
				"symbol %q expected %s/%s, got %s/%s",
				testCase.Symbol,
				testCase.From,
				testCase.To,
				fromCurrency,
				toCurrency,
			)
		}
	}
}

func TestCombinePricesAveragesExchanges(t *testing.T) {
	binancePrices := []CryptoPrice{
		{From: "BTC", To: "USD", Value: "100"},
		{From: "ETH", To: "USD", Value: "200"},
		{From: "ADA", To: "USD", Value: "1"},
		{From: "ADA", To: "USD", Value: "3"},
	}

	krakenPrices := []CryptoPrice{
		{From: "BTC", To: "USD", Value: "120"},
		{From: "ETH", To: "USD", Value: "240"},
		{From: "ADA", To: "USD", Value: "4"},
		{From: "XMR", To: "USD", Value: "150"},
	}

	got, err := combinePrices(binancePrices, krakenPrices)

	if err != nil {
		t.Fatalf("combinePrices returned error: %v", err)
	}

	want := []CryptoPrice{
		{From: "ADA", To: "USD", Value: "3"},
		{From: "BTC", To: "USD", Value: "110"},
		{From: "ETH", To: "USD", Value: "220"},
		{From: "XMR", To: "USD", Value: "150"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected combined prices\nwant: %#v\n got: %#v", want, got)
	}
}

func TestCollectMissingTickersReturnsSortedUniqueTickers(t *testing.T) {
	prices := []CryptoPrice{
		{From: "ETH", To: "USD", Value: "1"},
		{From: "BTC", To: "GBP", Value: "2"},
		{From: "ETH", To: "BTC", Value: "3"},
		{From: "XMR", To: "USD", Value: "4"},
	}

	currentTickerMap := map[string]bool{
		"BTC": true,
		"USD": true,
	}

	got := collectMissingTickers(prices, currentTickerMap)
	want := []string{"ETH", "GBP", "XMR"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected missing tickers\nwant: %#v\n got: %#v", want, got)
	}
}

func TestBuildPriceInsertRows(t *testing.T) {
	tickerMap := map[string]currencyInfo{
		"BTC": {Name: "BTC"},
		"ETH": {Name: "ETH"},
		"USD": {Name: "USD"},
	}

	t.Run("transforms and clamps prices", func(t *testing.T) {
		prices := []CryptoPrice{
			{From: "BTC", To: "USD", Value: "0"},
			{From: "ETH", To: "USD", Value: "-10"},
			{From: "ETH", To: "USD", Value: "12.5"},
		}

		rows, err := buildPriceInsertRows(prices, tickerMap)

		if err != nil {
			t.Fatalf("buildPriceInsertRows returned error: %v", err)
		}

		if len(rows) != 3 {
			t.Fatalf("expected 3 rows, got %d", len(rows))
		}

		if !rows[0].Value.Equal(VerySmallAmount) {
			t.Fatalf("expected first value %s, got %s", VerySmallAmount, rows[0].Value)
		}

		if !rows[1].Value.Equal(VerySmallAmount) {
			t.Fatalf("expected second value %s, got %s", VerySmallAmount, rows[1].Value)
		}

		wantThird, _ := decimal.NewFromString("12.5")
		if !rows[2].Value.Equal(wantThird) {
			t.Fatalf("expected third value %s, got %s", wantThird, rows[2].Value)
		}

		if rows[2].FromName != "ETH" || rows[2].ToName != "USD" {
			t.Fatalf("unexpected row names: %#v", rows[2])
		}
	})

	t.Run("returns error for invalid decimal", func(t *testing.T) {
		_, err := buildPriceInsertRows(
			[]CryptoPrice{{From: "BTC", To: "USD", Value: "not-a-number"}},
			tickerMap,
		)

		if err == nil {
			t.Fatal("expected decimal parse error")
		}
	})

	t.Run("returns error for missing currency info", func(t *testing.T) {
		_, err := buildPriceInsertRows(
			[]CryptoPrice{{From: "SOL", To: "USD", Value: "10"}},
			tickerMap,
		)

		if err == nil {
			t.Fatal("expected missing currency info error")
		}

		if !strings.Contains(err.Error(), "missing currency info for SOL") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
