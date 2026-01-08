package portfolio

import (
	"net/http"
	"sort"
	"strconv"
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
	currency_id,
	currency_ticker,
	currency_name,
	cash
from crypto_portfolio
where user_id = ?
order by updated_at desc
limit 1
`

func scanPortfolio(row database.Row, portfolio *model.Portfolio) error {
	var cash float64

	if err := row.Scan(
		&portfolio.Currency.ID,
		&portfolio.Currency.Ticker,
		&portfolio.Currency.Name,
		&cash,
	); err != nil {
		return err
	}

	portfolio.Cash = decimal.NewFromFloat(cash)

	return nil
}

func loadPortfolio(conn *database.Conn, user *model.User, portfolio *model.Portfolio) error {
	row := conn.QueryRow(portfolioQuery, user.ID)

	return scanPortfolio(row, portfolio)
}

var assetQuery = `
select
	currency_id,
	currency_ticker,
	currency_name,
	purchased,
	amount
from crypto_asset
where user_id = ? and is_deleted = 0
order by updated_at desc
limit 1 by currency_id
`

func scanAsset(row database.Row, asset *model.Asset) error {
	var purchased float64
	var amount float64

	if err := row.Scan(
		&asset.Currency.ID,
		&asset.Currency.Ticker,
		&asset.Currency.Name,
		&purchased,
		&amount,
	); err != nil {
		return err
	}

	asset.Purchased = decimal.NewFromFloat(purchased)
	asset.Amount = decimal.NewFromFloat(amount)

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
		assetQuery,
		userID,
	)
}

var priceQuery = `
select
	from_currency_id,
	from_currency_ticker,
	from_currency_name,
	to_currency_id,
	to_currency_ticker,
	to_currency_name,
	time,
	value
from crypto_currency_prices
`

func scanPrice(row database.Row, price *model.Price) error {
	var value float64

	if err := row.Scan(
		&price.From.ID,
		&price.From.Ticker,
		&price.From.Name,
		&price.To.ID,
		&price.To.Ticker,
		&price.To.Name,
		&price.Time,
		&value,
	); err != nil {
		return err
	}

	price.Value = decimal.NewFromFloat(value)

	return nil
}

func loadPriceList(conn *database.Conn, currency *model.Currency, tickerList []string, priceList *[]model.Price) error {
	if len(tickerList) == 0 {
		*priceList = nil

		return nil
	}

	return model.LoadList(
		conn,
		priceList,
		len(tickerList)*2,
		scanPrice,
		buildPriceQuery(len(tickerList)),
		buildPriceArgs(tickerList, currency.ID)...,
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
		if price.To.ID == currency.ID {
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
	(user_id, username, currency_id, currency_ticker, currency_name, purchased, amount, updated_at, is_deleted)
values (?, ?, ?, ?, ?, ?, ?, now64(9), 0)
`

func updateAsset(conn database.Queryable, user *model.User, asset *model.Asset) error {
	return conn.Exec(
		assetUpdateQuery,
		user.ID,
		user.Username,
		asset.Currency.ID,
		asset.Currency.Ticker,
		asset.Currency.Name,
		decimalToFloat(asset.Purchased),
		decimalToFloat(asset.Amount),
	)
}

var portfolioUpdateQuery = `
insert into crypto_portfolio
	(user_id, username, currency_id, currency_ticker, currency_name, cash, updated_at, is_deleted)
values (?, ?, ?, ?, ?, ?, now64(9), 0)
`

func updatePortfolio(conn database.Queryable, user *model.User, portfolio *model.Portfolio) error {
	return conn.Exec(
		portfolioUpdateQuery,
		user.ID,
		user.Username,
		portfolio.Currency.ID,
		portfolio.Currency.Ticker,
		portfolio.Currency.Name,
		decimalToFloat(portfolio.Cash),
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

	currencyID, err := strconv.ParseInt(request.Form.Get("currency"), 10, 64)

	if err != nil {
		util.RespondValidationError(writer, "Invalid currency ID")

		return
	}

	data.Portfolio.Cash, err = decimal.NewFromString(request.Form.Get("cash"))

	if err != nil {
		util.RespondValidationError(writer, "Invalid cash value")

		return
	}

	if data.Portfolio.Cash.IsNegative() {
		util.RespondValidationError(writer, "Cash must be non-negative")

		return
	}

	if err := query.LoadCurrencyByID(conn, &data.Portfolio.Currency, currencyID); err != nil {
		if err == database.ErrNoRows {
			util.RespondValidationError(writer, "Unknown currency ID")
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

	data.ToCurrencyList = query.BuildToCurrencyList(data.FromCurrencyList)

	if data.Portfolio.Currency.ID != 0 {
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
		`select currency_id, currency_ticker, currency_name, purchased, amount
		from crypto_asset
		where user_id = ? and currency_id = ? and is_deleted = 0
		order by updated_at desc
		limit 1`,
		data.User.ID,
		data.asset.Currency.ID,
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
	request *http.Request,
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
		`select currency_id, currency_ticker, currency_name, purchased, amount
		from crypto_asset
		where user_id = ? and currency_id = ? and is_deleted = 0
		order by updated_at desc
		limit 1`,
		data.User.ID,
		data.Asset.Currency.ID,
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

func decimalToFloat(value decimal.Decimal) float64 {
	floatValue, _ := value.Float64()

	return floatValue
}

func buildPriceQuery(tickerCount int) string {
	return priceQuery + `
	where from_currency_ticker in (` + makePlaceholders(tickerCount) + `)
		and (to_currency_id = ? or to_currency_ticker = 'BTC')
	order by time desc
	limit 1 by from_currency_ticker, to_currency_ticker`
}

func buildPriceArgs(tickerList []string, toCurrencyID int64) []any {
	args := make([]any, 0, len(tickerList)+1)

	for _, ticker := range tickerList {
		args = append(args, ticker)
	}

	args = append(args, toCurrencyID)

	return args
}

func makePlaceholders(count int) string {
	if count <= 0 {
		return ""
	}

	placeholders := make([]string, count)

	for i := 0; i < count; i++ {
		placeholders[i] = "?"
	}

	return strings.Join(placeholders, ", ")
}

func loadCurrencyByTicker(conn *database.Conn, currency *model.Currency, ticker string) error {
	row := conn.QueryRow("select id, ticker, name from crypto_currency where ticker = ?", ticker)

	return row.Scan(&currency.ID, &currency.Ticker, &currency.Name)
}
