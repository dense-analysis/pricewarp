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

func loadAssetList(conn *database.Conn, userID int, assetList *[]model.Asset) error {
	return model.LoadList(
		conn,
		assetList,
		1,
		scanAsset,
		assetQuery + "where user_id = $1 order by time",
		userID,
	)
}

func loadAsset(conn *database.Conn, userID int, ticker string, asset *model.Asset) error {
	row := conn.QueryRow(
		optionalAssetQuery + " where asset.user_id = $1 and currency.ticker = $2",
		userID,
		ticker,
	)

	return scanAsset(row, asset)
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
	AssetList []model.Asset
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
	}

	// TODO: Augment asset list with calculated price data
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

	if err := loadAsset(conn, data.User.ID, ticker, &asset); err != nil {
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
	Asset model.Asset
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

	if err := loadAsset(conn, data.User.ID, ticker, &data.Asset); err != nil {
		if err == database.ErrNoRows {
			util.RespondNotFound(writer)
		} else {
			util.RespondInternalServerError(writer, err)
		}

		return
	}

	// TODO: Augment asset with calculated price data
	// TODO: Render template.
}
