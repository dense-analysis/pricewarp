package portfolio

import (
	"net/http"
	"sort"
	"strings"

	"github.com/dense-analysis/pricewarp/internal/database"
	"github.com/dense-analysis/pricewarp/internal/model"
	"github.com/dense-analysis/pricewarp/internal/route/query"
	"github.com/dense-analysis/pricewarp/internal/route/util"
	"github.com/dense-analysis/pricewarp/internal/session"
	"github.com/dense-analysis/pricewarp/internal/template"
	"github.com/gorilla/mux"
	"github.com/shopspring/decimal"
)

var One decimal.Decimal = decimal.NewFromInt(1)
var Hundred decimal.Decimal = decimal.NewFromInt(100)

// TrackedAsset is an Asset with additional information available to it.
type TrackedAsset struct {
	model.Asset
	Value            decimal.Decimal
	ShareOfPortfolio decimal.Decimal
	Performance      decimal.Decimal
}

var portfolioQuery = `
select
	currency_ticker,
	currency_name,
	cash
from crypto_portfolio
where user_id = ?
order by updated_at desc
limit 1
`

func scanPortfolio(row database.Row, portfolio *model.Portfolio) error {
	var cash decimal.Decimal

	if err := row.Scan(
		&portfolio.Currency.Ticker,
		&portfolio.Currency.Name,
		&cash,
	); err != nil {
		return err
	}

	portfolio.Cash = cash

	return nil
}

func loadPortfolio(conn *database.Conn, user *model.User, portfolio *model.Portfolio) error {
	row := conn.QueryRow(portfolioQuery, user.ID)

	return scanPortfolio(row, portfolio)
}

func scanAsset(row database.Row, asset *model.Asset) error {
	var purchased decimal.Decimal
	var amount decimal.Decimal

	if err := row.Scan(
		&asset.Currency.Ticker,
		&asset.Currency.Name,
		&purchased,
		&amount,
	); err != nil {
		return err
	}

	asset.Purchased = purchased
	asset.Amount = amount

	return nil
}

func scanTrackedAsset(row database.Row, asset *TrackedAsset) error {
	return scanAsset(row, &asset.Asset)
}

func loadAssetList(conn *database.Conn, userID int64, assetList *[]TrackedAsset) error {
	return model.LoadList(
		conn,
		assetList,
		1,
		scanTrackedAsset,
		`
		select
			currency_ticker,
			currency_name,
			purchased,
			amount
		from crypto_asset
		where user_id = ?
		order by updated_at desc
		limit 1 by currency_ticker
		`,
		userID,
	)
}

func scanPrice(row database.Row, price *model.Price) error {
	return row.Scan(
		&price.From.Ticker,
		&price.From.Name,
		&price.To.Ticker,
		&price.To.Name,
		&price.Time,
		&price.Value,
	)
}

// loadPriceList Loads the latest list of prices given a list of tickers.
//
// The `currency.Ticker` will be used for the fiat currency conversions, and
// more prices may be loaded so we can convert through BTC, say via USD.
func loadPriceList(conn *database.Conn, currency *model.Currency, tickerList []string, priceList *[]model.Price) error {
	if len(tickerList) == 0 {
		*priceList = nil

		return nil
	}

	// Build args for the price query.
	args := make([]any, 0, len(tickerList)+1)

	for _, ticker := range tickerList {
		args = append(args, ticker)
	}

	args = append(args, currency.Ticker)

	return model.LoadList(
		conn,
		priceList,
		len(tickerList)*2,
		scanPrice,
		`
			SELECT
				from_currency_ticker,
				argMax(from_currency_name, time) AS from_currency_name,
				to_currency_ticker,
				argMax(to_currency_name, time) AS to_currency_name,
				max(time) AS latest_time,
				argMax(value, time) AS value
			FROM crypto_currency_prices
			-- Only get prices from at most 3 months back.
			-- This avoids fetching very old partitions for dead coins.
			-- We will report these prices as 0 below.
			PREWHERE yearmonth >= toYear(addMonths(now(), -3)) * 100 + toMonth(addMonths(now(), -3))
			WHERE from_currency_ticker in (`+makePlaceholders(len(tickerList))+`)
			AND (to_currency_ticker = ? or to_currency_ticker = 'BTC')
			GROUP BY from_currency_ticker, to_currency_ticker
		`,
		args...,
	)
}

func loadAssetPrices(conn *database.Conn, currency *model.Currency, assetList []TrackedAsset) error {
	tickerList := make([]string, 0, len(assetList)+1)
	tickerList = append(tickerList, "BTC")

	for _, asset := range assetList {
		tickerList = append(tickerList, asset.Currency.Ticker)
	}

	var priceList []model.Price

	if err := loadPriceList(conn, currency, tickerList, &priceList); err != nil {
		return err
	}

	btcPrices := map[string]decimal.Decimal{}
	currencyPrices := map[string]decimal.Decimal{}

	for _, price := range priceList {
		if price.To.Ticker == currency.Ticker {
			currencyPrices[price.From.Ticker] = price.Value
		} else {
			btcPrices[price.From.Ticker] = price.Value
		}
	}

	for i := range assetList {
		asset := &assetList[i]

		if multiplier, ok := currencyPrices[asset.Currency.Ticker]; ok {
			// Conversion from a currency to fiat directly.
			asset.Value = asset.Amount.Mul(multiplier)
		} else if toBtcMultiplier, ok := btcPrices[asset.Currency.Ticker]; ok {
			// Conversion from a currency to fiat via Bitcoin.
			if btcToCurrencyMultiplier, ok := currencyPrices["BTC"]; ok {
				asset.Value = asset.Amount.Mul(toBtcMultiplier).Mul(btcToCurrencyMultiplier)
			} else {
				asset.Value = decimal.Zero
			}
		} else {
			asset.Value = decimal.Zero
		}
	}

	totalValue := decimal.Zero

	for _, asset := range assetList {
		totalValue = totalValue.Add(asset.Value)
	}

	for i := range assetList {
		asset := &assetList[i]

		// The share of the portofolio is the value over the total value
		if totalValue.IsZero() {
			asset.ShareOfPortfolio = decimal.Zero
		} else {
			asset.ShareOfPortfolio = asset.Value.Div(totalValue).Mul(Hundred)
		}

		// Calculate percentage gains per asset
		if asset.Purchased.IsZero() {
			asset.Performance = decimal.Zero
		} else {
			asset.Performance = asset.Value.Div(asset.Purchased).Sub(One).Mul(Hundred)
		}
	}

	return nil
}

var assetUpdateQuery = `
insert into crypto_asset
	(user_id, username, currency_ticker, currency_name, purchased, amount, updated_at)
values (?, ?, ?, ?, ?, ?, now64(9))
`

func updateAsset(conn database.Queryable, user *model.User, asset *model.Asset) error {
	return conn.Exec(
		assetUpdateQuery,
		user.ID,
		user.Username,
		asset.Currency.Ticker,
		asset.Currency.Name,
		asset.Purchased,
		asset.Amount,
	)
}

var portfolioUpdateQuery = `
insert into crypto_portfolio
	(user_id, username, currency_ticker, currency_name, cash, updated_at)
values (?, ?, ?, ?, ?, now64(9))
`

func updatePortfolio(conn database.Queryable, user *model.User, portfolio *model.Portfolio) error {
	return conn.Exec(
		portfolioUpdateQuery,
		user.ID,
		user.Username,
		portfolio.Currency.Ticker,
		portfolio.Currency.Name,
		portfolio.Cash,
	)
}

func loadUser(conn *database.Conn, writer http.ResponseWriter, request *http.Request, user *model.User) bool {
	found, err := session.LoadUserFromSession(conn, request, user)

	if err != nil {
		util.RespondInternalServerError(writer, err)

		return false
	}

	return found
}

type PortfolioPageData struct {
	User      model.User
	Portfolio model.Portfolio
}

// HandlePortfolioUpdate updates the user's currency and balance of that currency.
func HandlePortfolioUpdate(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
	data := PortfolioPageData{}

	if !loadUser(conn, writer, request, &data.User) {
		util.RespondForbidden(writer)

		return
	}

	request.ParseForm()

	currencyTicker := request.Form.Get("currency")

	if currencyTicker == "" {
		util.RespondValidationError(writer, "Invalid currency ticker")

		return
	}

	var err error
	data.Portfolio.Cash, err = decimal.NewFromString(request.Form.Get("cash"))

	if err != nil {
		util.RespondValidationError(writer, "Invalid cash value")

		return
	}

	if data.Portfolio.Cash.IsNegative() {
		util.RespondValidationError(writer, "Cash must be non-negative")

		return
	}

	if err := query.LoadCurrencyByTicker(conn, &data.Portfolio.Currency, currencyTicker); err != nil {
		if err == database.ErrNoRows {
			util.RespondValidationError(writer, "Unknown currency ticker")
		} else {
			util.RespondInternalServerError(writer, err)
		}

		return
	}

	if err := updatePortfolio(conn, &data.User, &data.Portfolio); err != nil {
		util.RespondInternalServerError(writer, err)
	} else {
		http.Redirect(writer, request, "/portfolio", http.StatusFound)
	}
}

type PortfolioListPageData struct {
	PortfolioPageData
	AssetList          []TrackedAsset
	ToCurrencyList     []model.Currency
	FromCurrencyList   []model.Currency
	TotalPurchased     decimal.Decimal
	TotalValue         decimal.Decimal
	TotalProfit        decimal.Decimal
	AveragePerformance decimal.Decimal
}

type byValueOrder []TrackedAsset

func (a byValueOrder) Len() int {
	return len(a)
}

func (a byValueOrder) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a byValueOrder) Less(i, j int) bool {
	return a[j].Value.LessThan(a[i].Value)
}

// HandlePortfolio shows the assets and cash a user has.
func HandlePortfolio(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
	data := PortfolioListPageData{}

	if !loadUser(conn, writer, request, &data.User) {
		http.Redirect(writer, request, "/login", http.StatusFound)

		return
	}

	if err := loadPortfolio(conn, &data.User, &data.Portfolio); err != nil {
		if err != database.ErrNoRows {
			util.RespondInternalServerError(writer, err)

			return
		}
	}

	if err := query.LoadCurrencyList(conn, &data.FromCurrencyList); err != nil {
		util.RespondInternalServerError(writer, err)

		return
	}

	data.ToCurrencyList = query.GetToCurrencyList()

	if data.Portfolio.Currency.Ticker != "" {
		// Only load assets once a currency has been set.
		if err := loadAssetList(conn, data.User.ID, &data.AssetList); err != nil {
			util.RespondInternalServerError(writer, err)

			return
		}

		if err := loadAssetPrices(conn, &data.Portfolio.Currency, data.AssetList); err != nil {
			util.RespondInternalServerError(writer, err)

			return
		}

		sort.Sort(byValueOrder(data.AssetList))

		// Add cash in fiat to the total value and amount purchased.
		data.TotalValue = data.Portfolio.Cash
		data.TotalPurchased = data.Portfolio.Cash

		for _, asset := range data.AssetList {
			data.TotalValue = data.TotalValue.Add(asset.Value)
			data.TotalPurchased = data.TotalPurchased.Add(asset.Purchased)
		}

		data.TotalProfit = data.TotalValue.Sub(data.TotalPurchased)

		if data.TotalPurchased.IsZero() {
			data.AveragePerformance = decimal.Zero
		} else {
			data.AveragePerformance = data.TotalValue.Div(data.TotalPurchased).Sub(One).Mul(Hundred)
		}
	}

	template.Render(template.Portfolio, writer, data)
}

type AssetAdjustData struct {
	PortfolioPageData
	asset  model.Asset
	crypto decimal.Decimal
	fiat   decimal.Decimal
}

func loadAssetAdjustFormData(
	conn *database.Conn,
	data *AssetAdjustData,
	writer http.ResponseWriter,
	request *http.Request,
) bool {
	if !loadUser(conn, writer, request, &data.User) {
		util.RespondForbidden(writer)

		return false
	}

	if err := loadPortfolio(conn, &data.User, &data.Portfolio); err != nil {
		if err != database.ErrNoRows {
			util.RespondValidationError(writer, "Portfolio not configured")
		} else {
			util.RespondInternalServerError(writer, err)
		}

		return false
	}

	ticker := mux.Vars(request)["ticker"]

	if err := loadCurrencyByTicker(conn, &data.asset.Currency, ticker); err != nil {
		if err == database.ErrNoRows {
			util.RespondNotFound(writer)
		} else {
			util.RespondInternalServerError(writer, err)
		}

		return false
	}

	row := conn.QueryRow(
		`select currency_ticker, currency_name, purchased, amount
		from crypto_asset
		where user_id = ? and currency_ticker = ?
		order by updated_at desc
		limit 1`,
		data.User.ID,
		data.asset.Currency.Ticker,
	)

	if err := scanAsset(row, &data.asset); err != nil {
		if err != database.ErrNoRows {
			util.RespondInternalServerError(writer, err)

			return false
		}

		data.asset.Purchased = decimal.Zero
		data.asset.Amount = decimal.Zero
	}

	request.ParseForm()

	var err error
	data.fiat, err = decimal.NewFromString(request.Form.Get("fiat"))

	if err != nil {
		util.RespondValidationError(writer, "Invalid fiat value")

		return false
	}

	if data.fiat.IsNegative() {
		util.RespondValidationError(writer, "fiat must not be negative")

		return false
	}

	data.crypto, err = decimal.NewFromString(request.Form.Get("crypto"))

	if err != nil {
		util.RespondValidationError(writer, "Invalid crypto value")

		return false
	}

	if !data.crypto.IsPositive() {
		util.RespondValidationError(writer, "crypto must be positive")

		return false
	}

	return true
}

func saveAssetAdjustChanges(
	conn *database.Conn,
	data *AssetAdjustData,
	writer http.ResponseWriter,
	_ *http.Request,
) bool {
	if err := updateAsset(conn, &data.User, &data.asset); err != nil {
		util.RespondInternalServerError(writer, err)

		return false
	}

	if err := updatePortfolio(conn, &data.User, &data.Portfolio); err != nil {
		util.RespondInternalServerError(writer, err)

		return false
	}

	return true
}

// HandleAssetBuy swaps some cash for a cryptocurrency asset.
func HandleAssetBuy(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
	data := AssetAdjustData{}

	if loadAssetAdjustFormData(conn, &data, writer, request) {
		if data.fiat.GreaterThan(data.Portfolio.Cash) {
			util.RespondValidationError(writer, "You can't spend more fiat than you have")

			return
		}

		data.asset.Purchased = data.asset.Purchased.Add(data.fiat)
		data.asset.Amount = data.asset.Amount.Add(data.crypto)
		data.Portfolio.Cash = data.Portfolio.Cash.Sub(data.fiat)

		if saveAssetAdjustChanges(conn, &data, writer, request) {
			http.Redirect(writer, request, "/portfolio", http.StatusFound)
		}
	}
}

// HandleAssetSell swaps some cryptocurrency asset for cash.
func HandleAssetSell(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
	data := AssetAdjustData{}

	if loadAssetAdjustFormData(conn, &data, writer, request) {
		if data.crypto.GreaterThan(data.asset.Amount) {
			util.RespondValidationError(writer, "You can't remove more crypto than you have")

			return
		}

		// Subtract the cost by the average cost of the asset sold.
		differencePurchased := data.asset.Purchased.Mul(data.crypto.Div(data.asset.Amount))
		data.asset.Purchased = data.asset.Purchased.Sub(differencePurchased)
		data.asset.Amount = data.asset.Amount.Sub(data.crypto)
		data.Portfolio.Cash = data.Portfolio.Cash.Add(data.fiat)

		if saveAssetAdjustChanges(conn, &data, writer, request) {
			http.Redirect(writer, request, "/portfolio", http.StatusFound)
		}
	}
}

type AssetPageData struct {
	PortfolioPageData
	Asset TrackedAsset
}

// HandleAsset displays the details for a single Cryptocurrency asset.
func HandleAsset(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
	data := AssetPageData{}

	if !loadUser(conn, writer, request, &data.User) {
		http.Redirect(writer, request, "/login", http.StatusFound)

		return
	}

	if err := loadPortfolio(conn, &data.User, &data.Portfolio); err != nil {
		if err != database.ErrNoRows {
			util.RespondNotFound(writer)
		} else {
			util.RespondInternalServerError(writer, err)
		}

		return
	}

	ticker := mux.Vars(request)["ticker"]

	if err := loadCurrencyByTicker(conn, &data.Asset.Currency, ticker); err != nil {
		if err == database.ErrNoRows {
			util.RespondNotFound(writer)
		} else {
			util.RespondInternalServerError(writer, err)
		}

		return
	}

	row := conn.QueryRow(
		`select currency_ticker, currency_name, purchased, amount
		from crypto_asset
		where user_id = ? and currency_ticker = ?
		order by updated_at desc
		limit 1`,
		data.User.ID,
		data.Asset.Currency.Ticker,
	)

	assetList := make([]TrackedAsset, 1)

	if err := scanTrackedAsset(row, &assetList[0]); err != nil {
		if err != database.ErrNoRows {
			util.RespondInternalServerError(writer, err)

			return
		}

		assetList[0].Currency = data.Asset.Currency
		assetList[0].Purchased = decimal.Zero
		assetList[0].Amount = decimal.Zero
	}

	if err := loadAssetPrices(conn, &data.Portfolio.Currency, assetList); err != nil {
		util.RespondInternalServerError(writer, err)

		return
	}

	data.Asset = assetList[0]

	template.Render(template.Asset, writer, data)
}

func makePlaceholders(count int) string {
	if count <= 0 {
		return ""
	}

	placeholders := make([]string, count)

	for i := range count {
		placeholders[i] = "?"
	}

	return strings.Join(placeholders, ", ")
}

func loadCurrencyByTicker(conn *database.Conn, currency *model.Currency, ticker string) error {
	row := conn.QueryRow("select ticker, name from crypto_currencies where ticker = ?", ticker)

	return row.Scan(&currency.Ticker, &currency.Name)
}
