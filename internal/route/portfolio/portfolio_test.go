package portfolio

import (
	"errors"
	"testing"

	"github.com/dense-analysis/pricewarp/internal/model"
	"github.com/shopspring/decimal"
)

func decimalFromString(value string) decimal.Decimal {
	decimalValue, err := decimal.NewFromString(value)

	if err != nil {
		panic(err)
	}

	return decimalValue
}

func TestSwitchPortfolioCurrencyValuesConvertsCashAndPurchased(t *testing.T) {
	portfolioData := model.Portfolio{
		Currency: model.Currency{Ticker: "USD", Name: "USD"},
		Cash:     decimalFromString("1000"),
	}

	assetList := []model.Asset{
		{
			Currency:  model.Currency{Ticker: "ETH", Name: "Ethereum"},
			Purchased: decimalFromString("250"),
			Amount:    decimalFromString("1.5"),
		},
		{
			Currency:  model.Currency{Ticker: "SOL", Name: "Solana"},
			Purchased: decimalFromString("100"),
			Amount:    decimalFromString("5"),
		},
	}

	priceList := []model.Price{
		{
			From:  model.Currency{Ticker: "USD"},
			To:    model.Currency{Ticker: "BTC"},
			Value: decimalFromString("0.00002"),
		},
		{
			From:  model.Currency{Ticker: "BTC"},
			To:    model.Currency{Ticker: "GBP"},
			Value: decimalFromString("40000"),
		},
	}

	toCurrency := model.Currency{Ticker: "GBP", Name: "GBP"}

	err := switchPortfolioCurrencyValues(&portfolioData, assetList, &toCurrency, priceList)

	if err != nil {
		t.Fatalf("unexpected error switching currency: %v", err)
	}

	if !portfolioData.Cash.Equal(decimalFromString("800")) {
		t.Fatalf("expected cash to be 800 GBP, got %s", portfolioData.Cash.String())
	}

	if !assetList[0].Purchased.Equal(decimalFromString("200")) {
		t.Fatalf("expected first purchased value to be 200 GBP, got %s", assetList[0].Purchased.String())
	}

	if !assetList[1].Purchased.Equal(decimalFromString("80")) {
		t.Fatalf("expected second purchased value to be 80 GBP, got %s", assetList[1].Purchased.String())
	}
}

func TestSwitchPortfolioCurrencyValuesKeepsPerformancePercent(t *testing.T) {
	usd := model.Currency{Ticker: "USD", Name: "USD"}
	gbp := model.Currency{Ticker: "GBP", Name: "GBP"}
	eth := model.Currency{Ticker: "ETH", Name: "Ethereum"}

	beforeSwitchAssetList := []TrackedAsset{
		{
			Asset: model.Asset{
				Currency:  eth,
				Purchased: decimalFromString("100"),
				Amount:    decimalFromString("10"),
			},
		},
	}

	applyTrackedAssetPrices(&usd, beforeSwitchAssetList, []model.Price{{
		From:  eth,
		To:    usd,
		Value: decimalFromString("12"),
	}})

	beforePerformance := beforeSwitchAssetList[0].Performance

	portfolioData := model.Portfolio{Currency: usd, Cash: decimalFromString("50")}
	assetList := []model.Asset{{
		Currency:  eth,
		Purchased: decimalFromString("100"),
		Amount:    decimalFromString("10"),
	}}

	switchPriceList := []model.Price{
		{
			From:  model.Currency{Ticker: "USD"},
			To:    model.Currency{Ticker: "BTC"},
			Value: decimalFromString("0.00002"),
		},
		{
			From:  model.Currency{Ticker: "BTC"},
			To:    gbp,
			Value: decimalFromString("40000"),
		},
	}

	err := switchPortfolioCurrencyValues(&portfolioData, assetList, &gbp, switchPriceList)

	if err != nil {
		t.Fatalf("unexpected error switching currency: %v", err)
	}

	afterSwitchAssetList := []TrackedAsset{{Asset: assetList[0]}}

	applyTrackedAssetPrices(&gbp, afterSwitchAssetList, []model.Price{
		{
			From:  eth,
			To:    model.Currency{Ticker: "BTC"},
			Value: decimalFromString("0.00024"),
		},
		{
			From:  model.Currency{Ticker: "BTC"},
			To:    gbp,
			Value: decimalFromString("40000"),
		},
	})

	afterPerformance := afterSwitchAssetList[0].Performance

	if !afterPerformance.Equal(beforePerformance) {
		t.Fatalf("expected performance to stay at %s%%, got %s%%", beforePerformance.String(), afterPerformance.String())
	}

	if !afterSwitchAssetList[0].CurrentPrice.Equal(decimalFromString("9.6")) {
		t.Fatalf("expected current price to be 9.6 GBP, got %s", afterSwitchAssetList[0].CurrentPrice.String())
	}
}

func TestSwitchPortfolioCurrencyValuesFailsWithoutConversionRate(t *testing.T) {
	portfolioData := model.Portfolio{
		Currency: model.Currency{Ticker: "USD", Name: "USD"},
		Cash:     decimalFromString("100"),
	}

	toCurrency := model.Currency{Ticker: "EUR", Name: "EUR"}

	err := switchPortfolioCurrencyValues(&portfolioData, nil, &toCurrency, nil)

	if !errors.Is(err, ErrMissingCurrencyConversion) {
		t.Fatalf("expected missing conversion error, got %v", err)
	}
}
