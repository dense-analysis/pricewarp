// Package alert defines routes for alerts
package alert

import (
	"net/http"
	"strconv"

	"github.com/dense-analysis/pricewarp/internal/database"
	"github.com/dense-analysis/pricewarp/internal/model"
	"github.com/dense-analysis/pricewarp/internal/route/query"
	"github.com/dense-analysis/pricewarp/internal/route/util"
	"github.com/dense-analysis/pricewarp/internal/session"
	"github.com/dense-analysis/pricewarp/internal/template"
	"github.com/gorilla/mux"
	"github.com/shopspring/decimal"
)

var alertQuery = `
select
	alert_id,
	above,
	alert_time,
	sent,
	value,
	from_currency_ticker,
	from_currency_name,
	to_currency_ticker,
	to_currency_name,
	is_deleted
from (
	select
		alert_id,
		above,
		alert_time,
		sent,
		value,
		from_currency_ticker,
		from_currency_name,
		to_currency_ticker,
		to_currency_name,
		is_deleted
	from crypto_alert
	where user_id = ?
	order by updated_at desc
	limit 1 by alert_id
)
`

func scanAlert(row database.Row, alert *model.Alert) error {
	var value decimal.Decimal
	var above uint8
	var sent uint8
	var isDeleted uint8

	if err := row.Scan(
		&alert.ID,
		&above,
		&alert.Time,
		&sent,
		&value,
		&alert.From.Ticker,
		&alert.From.Name,
		&alert.To.Ticker,
		&alert.To.Name,
		&isDeleted,
	); err != nil {
		return err
	}

	if isDeleted == 1 {
		return database.ErrNoRows
	}

	alert.Above = above == 1
	alert.Sent = sent == 1
	alert.Value = value
	alert.From.ID = database.HashID(alert.From.Ticker)
	alert.To.ID = database.HashID(alert.To.Ticker)

	return nil
}

var currencyQuery = `select ticker, name from crypto_currencies `

func scanCurrency(row database.Row, currency *model.Currency) error {
	if err := row.Scan(&currency.Ticker, &currency.Name); err != nil {
		return err
	}

	currency.ID = database.HashID(currency.Ticker)

	return nil
}

func loadAlertList(conn *database.Conn, userID int64, alertList *[]model.Alert) error {
	return model.LoadList(
		conn,
		alertList,
		1,
		scanAlert,
		alertQuery+"where is_deleted = 0 order by alert_time",
		userID,
	)
}

func loadCurrencyList(conn *database.Conn, currencyList *[]model.Currency) error {
	return model.LoadList(
		conn,
		currencyList,
		500,
		scanCurrency,
		currencyQuery+"order by name",
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

type AlertPageData struct {
	User             model.User
	Alert            model.Alert
	FromCurrencyList []model.Currency
	ToCurrencyList   []model.Currency
}

type AlertListPageData struct {
	AlertPageData
	AlertList []model.Alert
}

func HandleAlertList(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
	data := AlertListPageData{}
	data.Alert.Above = true

	if !loadUser(conn, writer, request, &data.User) {
		http.Redirect(writer, request, "/login", http.StatusFound)

		return
	}

	if err := loadAlertList(conn, data.User.ID, &data.AlertList); err != nil {
		util.RespondInternalServerError(writer, err)

		return
	}

	if err := loadCurrencyList(conn, &data.FromCurrencyList); err != nil {
		util.RespondInternalServerError(writer, err)

		return
	}

	data.ToCurrencyList = query.BuildToCurrencyList(data.FromCurrencyList)
	template.Render(template.AlertList, writer, data)
}

func loadAlertForRequest(
	conn *database.Conn,
	writer http.ResponseWriter,
	request *http.Request,
	user *model.User,
	alert *model.Alert,
) bool {
	alertID, err := strconv.ParseInt(mux.Vars(request)["id"], 10, 64)

	if err != nil {
		util.RespondNotFound(writer)

		return false
	}

	row := conn.QueryRow(
		`select
			alert_id,
			above,
			alert_time,
			sent,
			value,
			from_currency_ticker,
			from_currency_name,
			to_currency_ticker,
			to_currency_name,
			is_deleted
		from crypto_alert
		where user_id = ? and alert_id = ?
		order by updated_at desc
		limit 1`,
		user.ID,
		alertID,
	)

	if err := scanAlert(row, alert); err != nil {
		if err == database.ErrNoRows {
			util.RespondNotFound(writer)
		} else {
			util.RespondInternalServerError(writer, err)
		}

		return false
	}

	return true
}

func HandleAlert(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
	data := AlertPageData{}
	data.Alert.Above = true

	if !loadUser(conn, writer, request, &data.User) {
		http.Redirect(writer, request, "/login", http.StatusFound)

		return
	}

	if loadAlertForRequest(conn, writer, request, &data.User, &data.Alert) {
		if err := loadCurrencyList(conn, &data.FromCurrencyList); err != nil {
			util.RespondInternalServerError(writer, err)
		} else {
			data.ToCurrencyList = query.BuildToCurrencyList(data.FromCurrencyList)
			template.Render(template.Alert, writer, data)
		}
	}
}

func loadAlertFromForm(
	conn *database.Conn,
	writer http.ResponseWriter,
	request *http.Request,
	alert *model.Alert,
) bool {
	var err error
	request.ParseForm()

	fromTicker := request.Form.Get("from")
	toTicker := request.Form.Get("to")

	if fromTicker == "" || toTicker == "" {
		util.RespondValidationError(writer, "Invalid currency ticker")

		return false
	}

	if fromTicker == toTicker {
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

	row = conn.QueryRow(currencyQuery+"where ticker = ?", fromTicker)

	if err := scanCurrency(row, &alert.From); err != nil {
		util.RespondInternalServerError(writer, err)

		return false
	}

	row = conn.QueryRow(currencyQuery+"where ticker = ?", toTicker)

	if err := scanCurrency(row, &alert.To); err != nil {
		util.RespondInternalServerError(writer, err)

		return false
	}

	return true
}

func HandleSubmitAlert(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
	var user model.User
	var alert model.Alert

	if !loadUser(conn, writer, request, &user) {
		util.RespondForbidden(writer)

		return
	}

	if loadAlertFromForm(conn, writer, request, &alert) {
		alertID, err := database.RandomID()

		if err != nil {
			util.RespondInternalServerError(writer, err)

			return
		}

		insertSQL := `
		insert into crypto_alert
			(alert_id, user_id, username, above, alert_time, sent, value,
			 from_currency_ticker, from_currency_name,
			 to_currency_ticker, to_currency_name,
			 updated_at, is_deleted)
		values (?, ?, ?, ?, now64(9), 0, ?,
			?, ?,
			?, ?,
			now64(9), 0)
		`

		if err := conn.Exec(
			insertSQL,
			alertID,
			user.ID,
			user.Username,
			boolToUint(alert.Above),
			alert.Value,
			alert.From.Ticker,
			alert.From.Name,
			alert.To.Ticker,
			alert.To.Name,
		); err != nil {
			util.RespondInternalServerError(writer, err)
		} else {
			http.Redirect(writer, request, "/alert", http.StatusFound)
		}
	}
}

func HandleUpdateAlert(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
	var user model.User
	var alert model.Alert

	if !loadUser(conn, writer, request, &user) {
		util.RespondForbidden(writer)

		return
	}

	if loadAlertForRequest(conn, writer, request, &user, &alert) && loadAlertFromForm(conn, writer, request, &alert) {
		updateSQL := `
		insert into crypto_alert
			(alert_id, user_id, username, above, alert_time, sent, value,
			 from_currency_ticker, from_currency_name,
			 to_currency_ticker, to_currency_name,
			 updated_at, is_deleted)
		values (?, ?, ?, ?, now64(9), 0, ?,
			?, ?,
			?, ?,
			now64(9), 0)
		`

		if err := conn.Exec(
			updateSQL,
			alert.ID,
			user.ID,
			user.Username,
			boolToUint(alert.Above),
			alert.Value,
			alert.From.Ticker,
			alert.From.Name,
			alert.To.Ticker,
			alert.To.Name,
		); err != nil {
			util.RespondInternalServerError(writer, err)
		} else {
			http.Redirect(writer, request, "/alert", http.StatusFound)
		}
	}
}

func HandleDeleteAlert(conn *database.Conn, writer http.ResponseWriter, request *http.Request) {
	var user model.User
	var alert model.Alert

	if !loadUser(conn, writer, request, &user) {
		util.RespondForbidden(writer)

		return
	}

	if loadAlertForRequest(conn, writer, request, &user, &alert) {
		deleteSQL := `
		insert into crypto_alert
			(alert_id, user_id, username, above, alert_time, sent, value,
			 from_currency_ticker, from_currency_name,
			 to_currency_ticker, to_currency_name,
			 updated_at, is_deleted)
		values (?, ?, ?, ?, ?, ?, ?,
			?, ?,
			?, ?,
			now64(9), 1)
		`

		if err := conn.Exec(
			deleteSQL,
			alert.ID,
			user.ID,
			user.Username,
			boolToUint(alert.Above),
			alert.Time,
			boolToUint(alert.Sent),
			alert.Value,
			alert.From.Ticker,
			alert.From.Name,
			alert.To.Ticker,
			alert.To.Name,
		); err != nil {
			util.RespondInternalServerError(writer, err)
		} else {
			writer.WriteHeader(http.StatusNoContent)
		}
	}
}

func boolToUint(value bool) uint8 {
	if value {
		return 1
	}

	return 0
}
