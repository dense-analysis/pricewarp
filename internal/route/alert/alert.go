// Package alert defines routes for alerts
package alert

import (
	"fmt"
	"strconv"
	"net/http"
	"github.com/shopspring/decimal"
	"github.com/gorilla/mux"
	"github.com/w0rp/pricewarp/internal/template"
	"github.com/w0rp/pricewarp/internal/database"
	"github.com/w0rp/pricewarp/internal/model"
	"github.com/w0rp/pricewarp/internal/session"
	"github.com/w0rp/pricewarp/internal/route/util"
)

type FallbackUserOption bool

const (
	RedirectIfNoUser FallbackUserOption = false
	ForbiddenIfNoUser                   = true
)

var alertQuery = `
select
	crypto_alert.id,
	above,
	time,
	sent,
	value,
	from_currency.id,
	from_currency.ticker,
	from_currency.name,
	to_currency.id,
	to_currency.ticker,
	to_currency.name
from crypto_alert
inner join crypto_currency as from_currency
on from_currency.id = crypto_alert."from"
inner join crypto_currency as to_currency
on to_currency.id = crypto_alert."to"
`

func ScanAlert(row database.Row, alert *model.Alert) error {
	return row.Scan(
		&alert.ID,
		&alert.Above,
		&alert.Time,
		&alert.Sent,
		&alert.Value,
		&alert.From.ID,
		&alert.From.Ticker,
		&alert.From.Name,
		&alert.To.ID,
		&alert.To.Ticker,
		&alert.To.Name,
	)
}

var currencyQuery = `select id, ticker, name from crypto_currency `

func ScanCurrency(row database.Row, currency *model.Currency) error {
	return row.Scan(&currency.ID, &currency.Ticker, &currency.Name)
}

func loadAlertList(conn *database.Conn, userID int, alertList *[]model.Alert) error {
	rows, err := conn.Query(alertQuery + "where user_id = $1 order by time", userID)

	if err != nil {
		return err
	}

	*alertList = make([]model.Alert, 0, 1)
	var alert model.Alert

	for rows.Next() {
		if err := ScanAlert(rows, &alert); err != nil {
			return err
		}

		*alertList = append(*alertList, alert)
	}

	return nil
}

func loadCurrencyList(conn *database.Conn, currencyList *[]model.Currency) error {
	conn, err := database.Connect()

	if err != nil {
		return err
	}

	defer conn.Close()

	*currencyList = make([]model.Currency, 0, 500)

	rows, err := conn.Query(currencyQuery + "order by name")

	if err != nil {
		return err
	}

	var currency model.Currency

	for rows.Next() {
		if err := ScanCurrency(rows, &currency); err != nil {
			return err
		}

		*currencyList = append(*currencyList, currency)
	}

	return nil
}

func requireUser(writer http.ResponseWriter, request *http.Request, fallback FallbackUserOption) *model.User {
	user, err := session.LoadUserFromSession(request)

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(writer, "database connection error\n")

		return nil
	}

	if user != nil {
		return user
	}

	if fallback == RedirectIfNoUser {
		http.Redirect(writer, request, "/login", http.StatusFound)
	} else {
		writer.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(writer, "403: Forbidden\n")
	}

	return nil
}

type AlertPageData struct {
	Alert model.Alert
	FromCurrencyList []model.Currency
	ToCurrencyList []model.Currency
}

type AlertListPageData struct {
	AlertPageData
	AlertList []model.Alert
}

func HandleAlertList(writer http.ResponseWriter, request *http.Request) {
	user := requireUser(writer, request, RedirectIfNoUser)

	if user == nil {
		return
	}

	data := AlertListPageData{}
	data.Alert.Above = true

	conn, err := database.Connect()

	if err != nil {
		util.RespondInternalServerError(writer, err)

		return
	}

	defer conn.Close()

	if err := loadAlertList(conn, user.ID, &data.AlertList); err != nil {
		util.RespondInternalServerError(writer, err)

		return
	}

	if err := loadCurrencyList(conn, &data.FromCurrencyList); err != nil {
		util.RespondInternalServerError(writer, err)

		return
	}

	data.ToCurrencyList = data.FromCurrencyList

	template.Render(template.AlertList, writer, data)
}

func loadAlertForRequest(writer http.ResponseWriter, request *http.Request, alert *model.Alert) bool {
	user := requireUser(writer, request, RedirectIfNoUser)

	if user == nil {
		return false
	}

	alertID, err := strconv.Atoi(mux.Vars(request)["id"])

	if err != nil {
		util.RespondNotFound(writer)

		return false
	}

	conn, err := database.Connect()

	defer conn.Close()

	row := conn.QueryRow(alertQuery + " where user_id = $1 and crypto_alert.id = $2", user.ID, alertID)

	if err := ScanAlert(row, alert); err != nil {
		if err == database.ErrNoRows {
			util.RespondNotFound(writer)
		} else {
			util.RespondInternalServerError(writer, err)
		}

		return false
	}

	return true
}

func HandleAlert(writer http.ResponseWriter, request *http.Request) {
	user := requireUser(writer, request, RedirectIfNoUser)

	if user == nil {
		return
	}

	data := AlertPageData{}
	data.Alert.Above = true

	if !loadAlertForRequest(writer, request, &data.Alert) {
		return
	}

	conn, err := database.Connect()

	if err != nil {
		util.RespondInternalServerError(writer, err)

		return
	}

	defer conn.Close()

	if err := loadCurrencyList(conn, &data.FromCurrencyList); err != nil {
		util.RespondInternalServerError(writer, err)

		return
	}

	data.ToCurrencyList = data.FromCurrencyList

	template.Render(template.AlertList, writer, data)
}

func loadAlertFromRequest(alert *model.Alert, writer http.ResponseWriter, request *http.Request) bool {
	var err error
	request.ParseForm()

	from, err := strconv.Atoi(request.Form.Get("from"))

	if err != nil {
		util.RespondValidationError(writer, "Invalid from currency ID")

		return false
	}

	to, err := strconv.Atoi(request.Form.Get("to"))

	if err != nil {
		util.RespondValidationError(writer, "Invalid to currency ID")

		return false
	}

	if from == to {
		util.RespondValidationError(writer, "From and to currencies cannot be the same")

		return false
	}

	value, err := decimal.NewFromString(request.Form.Get("value"))

	if err != nil {
		util.RespondValidationError(writer, "Invalid value")

		return false
	}

	direction := request.Form.Get("direction")

	if direction != "above" && direction != "below" {
		util.RespondValidationError(writer, "Invalid direction")

		return false
	}

	alert.Value = value

	if direction == "above" {
		alert.Above = true
	} else {
		alert.Above = false
	}

	var row database.Row

	conn, err := database.Connect()

	if err != nil {
		util.RespondInternalServerError(writer, err)

		return false
	}

	defer conn.Close()

	row = conn.QueryRow(currencyQuery + "where id = $1", from)

	if err := ScanCurrency(row, &alert.From); err != nil {
		util.RespondInternalServerError(writer, err)

		return false
	}

	row = conn.QueryRow(currencyQuery + "where id = $1", to)

	if err := ScanCurrency(row, &alert.To); err != nil {
		util.RespondInternalServerError(writer, err)

		return false
	}

	return true
}

func HandleSubmitAlert(writer http.ResponseWriter, request *http.Request) {
	user := requireUser(writer, request, ForbiddenIfNoUser)

	if user == nil {
		return
	}

	var alert model.Alert

	if loadAlertFromRequest(&alert, writer, request) {
		insertSQL := `
		insert into crypto_alert(user_id, above, time, sent, value, "from", "to")
		values ($1, $2, NOW(), false, $3, $4, $5)
		`

		conn, err := database.Connect()

		if err != nil {
			util.RespondInternalServerError(writer, err)

			return
		}

		defer conn.Close()

		if err := conn.Exec(insertSQL, user.ID, alert.Above, alert.Value, alert.From.ID, alert.To.ID); err != nil {
			util.RespondInternalServerError(writer, err)

			return
		}

		http.Redirect(writer, request, "/alert", http.StatusFound)
	}
}

func HandleUpdateAlert(writer http.ResponseWriter, request *http.Request) {
	var alert model.Alert

	if !loadAlertForRequest(writer, request, &alert) {
		return
	}

	if loadAlertFromRequest(&alert, writer, request) {
		updateSQL := `
		update crypto_alert
		set above = $2,
			time = NOW(),
			sent = false,
			value = $3,
			"from" = $4,
			"to" = $5
		where id = $1
		`

		conn, err := database.Connect()

		if err != nil {
			util.RespondInternalServerError(writer, err)

			return
		}

		defer conn.Close()

		if err := conn.Exec(updateSQL, alert.ID, alert.Above, alert.Value, alert.From.ID, alert.To.ID); err != nil {
			util.RespondInternalServerError(writer, err)

			return
		}

		http.Redirect(writer, request, "/alert", http.StatusFound)
	}
}

func HandleDeleteAlert(writer http.ResponseWriter, request *http.Request) {
	var alert model.Alert

	if !loadAlertForRequest(writer, request, &alert) {
		return
	}

	conn, err := database.Connect()

	if err != nil {
		util.RespondInternalServerError(writer, err)

		return
	}

	defer conn.Close()

	err = conn.Exec("delete from crypto_alert where id = $1", alert.ID)

	if err != nil {
		util.RespondInternalServerError(writer, err)

		return
	}

	writer.WriteHeader(http.StatusNoContent)
}
