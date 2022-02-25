package portfolio

import (
	"github.com/w0rp/pricewarp/internal/database"
	"net/http"
)

// HandlePortfolioUpdate updates the user's currency and balance of that currency.
func HandlePortfolioUpdate(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
}

// HandlePortfolioList shows the assets and cash a user has.
func HandlePortfolioList(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
}

// HandlePortfolioBuy swaps some cash for a cryptocurrency asset.
func HandlePortfolioBuy(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
}

// HandlePortfolioSell swaps some cryptocurrency asset for cash.
func HandlePortfolioSell(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
}

// HandlePortfolioAsset displays the details for a single Cryptocurrency asset.
func HandlePortfolioAsset(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
}
