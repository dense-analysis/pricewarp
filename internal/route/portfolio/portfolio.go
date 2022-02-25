package portfolio

import (
	"strconv"
	"github.com/shopspring/decimal"
	"github.com/gorilla/mux"
	"github.com/w0rp/pricewarp/internal/database"
	"github.com/w0rp/pricewarp/internal/session"
	"github.com/w0rp/pricewarp/internal/model"
	"github.com/w0rp/pricewarp/internal/route/util"
	"net/http"
)

// TrackedAsset is an Asset with additional information available to it.
type TrackedAsset struct {
	model.Asset
	Value decimal.Decimal
	ShareOfPortfolio decimal.Decimal
	Performance decimal.Decimal
}

var portfolioQuery = `
select
	currency.id,
	currency.ticker,
	currency.name,
	cash
from crypto_portfolio
`

func scanPortfolio(row database.Row, portfolio *model.Portfolio) error {
	return row.Scan(
		portfolio.Currency.ID,
		portfolio.Currency.Ticker,
		portfolio.Currency.Name,
		portfolio.Cash,
	)
}

func loadPortfolio(conn *database.Conn, user *model.User, portfolio *model.Portfolio) error {
	row := conn.QueryRow(portfolioQuery + " where user_id = $1", user.ID)

	return scanPortfolio(row, portfolio)
}

var currencyQuery = "select id, ticker, name from crypto_currency"

func scanCurrency(row database.Row, currency *model.Currency) error {
	return row.Scan(&currency.ID, &currency.Ticker, &currency.Name)
}

var assetQuery = `
select
	currency.id,
	currency.ticker,
	currency.name,
	purchased,
	amount
from crypto_asset
inner join crypto_currency as currency
on currency.id = crypto_asset."currency_id"
`

var optionalAssetQuery = `
select
	currency.id,
	currency.ticker,
	currency.name,
	coalesce(asset.purchased, 0::numeric),
	coalesce(asset.amount, 0::numeric)
from crypto_currency as currency
left join crypto_asset as asset
on asset.currency_id = currency.id
`

func scanAsset(row database.Row, asset *model.Asset) error {
	return row.Scan(
		&asset.Currency.ID,
		&asset.Currency.Ticker,
		&asset.Currency.Name,
		&asset.Purchased,
		&asset.Amount,
	)
}

func scanTrackedAsset(row database.Row, asset *TrackedAsset) error {
	return scanAsset(row, &asset.Asset)
}

func loadAssetList(conn *database.Conn, userID int, assetList *[]TrackedAsset) error {
	return model.LoadList(
		conn,
		assetList,
		1,
		scanTrackedAsset,
		assetQuery + "where user_id = $1 order by time",
		userID,
	)
}

var priceQuery = `
select distinct on("from", "to")
	from_currency.id,
	from_currency.ticker,
	from_currency.name,
	to_currency.id,
	to_currency.ticker,
	to_currency.name,
	time,
	value
from crypto_price
inner join crypto_currency as from_currency
on from_currency.id = crypto_price."from"
inner join crypto_currency as to_currency
on to_currency.id = crypto_price."to"
`

func scanPrice(row database.Row, price *model.Price) error {
	return row.Scan(
		&price.From.ID,
		&price.From.Ticker,
		&price.From.Name,
		&price.To.ID,
		&price.To.Ticker,
		&price.To.Name,
		&price.Time,
		&price.Value,
	)
}

func loadPriceList(conn *database.Conn, currency *model.Currency, tickerList []string, priceList *[]model.Price) error {
	if len(tickerList) == 0 {
		*priceList = nil

		return nil
	}

	return model.LoadList(
		conn,
		priceList,
		len(tickerList) * 2,
		scanPrice,
		priceQuery + `
			where from_currency.ticker = ANY($1)
			and (to_currency.id = $2 or to_currency.ticker = 'BTC')"
			order by "from" desc, "to" desc, time desc;
		`,
		tickerList,
		currency.ID,
	)
}

func loadAssetPrices(conn *database.Conn, currency *model.Currency, assetList []TrackedAsset) error {
	tickerList := make([]string, 0, len(assetList) + 1)
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

	for _, asset := range assetList {
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

	one := decimal.New(1, 1)
	hundred := decimal.New(100, 1)

	for _, asset := range assetList {
		// The share of the portofolio is the value over the total value
		if totalValue.IsZero() {
			asset.ShareOfPortfolio = decimal.Zero
		} else {
			asset.ShareOfPortfolio = asset.Value.Div(totalValue).Mul(hundred)
		}

		// Calculate percentage gains per asset
		if asset.Purchased.IsZero() {
			asset.Performance = decimal.Zero
		} else {
			asset.Performance = asset.Value.Div(asset.Purchased).Sub(one).Mul(hundred)
		}
	}

	return nil
}

var assetUpdateQuery = `
update crypto_asset
set purchased = $3, amount = $4
where user_id = $1 and currency_id = $2
`

func updateAsset(conn database.Queryable, user *model.User, asset *model.Asset) error {
	return conn.Exec(assetUpdateQuery, user.ID, asset.Currency.ID, asset.Purchased, asset.Amount)
}

var portfolioUpdateQuery = `
insert into crypto_portfolio (user_id, currency_id, cash)
values ($1, $2, $3)
on conflict (user_id, currency_id) do update
set cash = $3
`

func updatePortfolio(conn database.Queryable, user *model.User, portfolio *model.Portfolio) error {
	return conn.Exec(portfolioUpdateQuery, user.ID, portfolio.Currency.ID, portfolio.Cash)
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
	User model.User
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

	currencyID, err := strconv.Atoi(request.Form.Get("currency"))

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

	row := conn.QueryRow(currencyQuery + " where currency_id = $1", currencyID)

	if err := scanCurrency(row, &data.Portfolio.Currency); err != nil {
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
	AssetList []TrackedAsset
	TotalValue decimal.Decimal
	AveragePerformance decimal.Decimal
}

// HandlePortfolioList shows the assets and cash a user has.
func HandlePortfolioList(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
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

		data.TotalValue = decimal.Zero
		totalPurchased := decimal.Zero

		for _, asset := range data.AssetList {
			data.TotalValue = data.TotalValue.Add(asset.Value)
			totalPurchased = totalPurchased.Add(asset.Purchased)
		}

		if totalPurchased.IsZero() {
			data.AveragePerformance = decimal.Zero
		} else {
			data.AveragePerformance = data.TotalValue.
				Div(totalPurchased).
				Sub(decimal.New(1, 1)).
				Mul(decimal.New(100, 1))
		}
	}

	// TODO: Render template.
}

type AssetAdjustData struct {
	PortfolioPageData
	asset model.Asset
	crypto decimal.Decimal
	fiat decimal.Decimal
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

	asset := model.Asset{}

	row := conn.QueryRow(
		optionalAssetQuery + " where asset.user_id = $1 and currency.ticker = $2",
		data.User.ID,
		ticker,
	)

	if err := scanAsset(row, &asset); err != nil {
		if err == database.ErrNoRows {
			util.RespondNotFound(writer)
		} else {
			util.RespondInternalServerError(writer, err)
		}

		return false
	}

	request.ParseForm()

	var err error
	data.fiat, err = decimal.NewFromString(request.Form.Get("fiat"))

	if err != nil {
		util.RespondValidationError(writer, "Invalid fiat value")

		return false
	}

	if !data.fiat.IsPositive() {
		util.RespondValidationError(writer, "fiat must be positive")

		return false
	}

	data.crypto, err = decimal.NewFromString(request.Form.Get("crypto"))

	if err != nil {
		util.RespondValidationError(writer, "Invalid crypto value")

		return false
	}

	if !data.fiat.IsPositive() {
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
	tx, err := conn.Begin()

	if err != nil {
		util.RespondInternalServerError(writer, err)

		return false
	}

	defer tx.Rollback()

	if err := updateAsset(tx, &data.User, &data.asset); err != nil {
		util.RespondInternalServerError(writer, err)

		return false
	}

	if err := updatePortfolio(tx, &data.User, &data.Portfolio); err != nil {
		util.RespondInternalServerError(writer, err)

		return false
	}

	tx.Commit()

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

		if (saveAssetAdjustChanges(conn, &data, writer, request)) {
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

		if (saveAssetAdjustChanges(conn, &data, writer, request)) {
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

	row := conn.QueryRow(
		optionalAssetQuery + " where asset.user_id = $1 and currency.ticker = $2",
		data.User.ID,
		ticker,
	)

	assetList := make([]TrackedAsset, 1)

	if err := scanTrackedAsset(row, &assetList[0]); err != nil {
		if err == database.ErrNoRows {
			util.RespondNotFound(writer)
		} else {
			util.RespondInternalServerError(writer, err)
		}

		return
	}

	if err := loadAssetPrices(conn, &data.Portfolio.Currency, assetList); err != nil {
		util.RespondInternalServerError(writer, err)

		return
	}

	data.Asset = assetList[0]

	// TODO: Render template.
}
