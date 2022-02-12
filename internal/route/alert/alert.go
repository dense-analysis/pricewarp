// Package alert defines routes for alerts
package alert

import (
	"fmt"
	"strconv"
	"strings"
	"html"
	"net/http"
	"github.com/shopspring/decimal"
	"github.com/gorilla/mux"
	"github.com/w0rp/pricewarp/internal/database"
	"github.com/w0rp/pricewarp/internal/model"
	"github.com/w0rp/pricewarp/internal/session"
	"github.com/w0rp/pricewarp/internal/route/util"
)

var htmlTemplate = `<!DOCTYPE html>
<html lang="en">
	<head>
		<meta charset="UTF-8">
		<title>Pricewarp</title>
	</head>
	<body>
		<div>
			<button id="logout" type="button">Logout</button>
		</div>
		{htmlBody}
		<script>
			document.getElementById("logout")
				.addEventListener("click", () => {
					fetch("/logout", {
						method: "POST",
					})
						.then(() => {
							window.location.assign("/")
						})
				})
		</script>
	</body>
</html>
`

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

func loadAlertList(userID int) ([]model.Alert, error) {
	conn, err := database.Connect()

	if err != nil {
		return nil, err
	}

	defer conn.Close()

	rows, err := conn.Query(alertQuery + "where user_id = $1 order by time", userID)

	if err != nil {
		return nil, err
	}

	alertList := make([]model.Alert, 0, 1)
	var alert model.Alert

	for rows.Next() {
		if err := ScanAlert(rows, &alert); err != nil {
			return nil, err
		}

		alertList = append(alertList, alert)
	}

	return alertList, nil
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

func buildCurrencyOptionsHtml(currencyList []model.Currency, selectedID int) string {
	var optionsHtml string

	for _, currency := range currencyList {
		selected := ""

		if currency.ID == selectedID {
			selected = " selected"
		}

		optionsHtml += fmt.Sprintf(
			"<option value=\"%d\"%s>%s</option>\n",
			currency.ID,
			selected,
			html.EscapeString(currency.Name),
		)
	}

	return optionsHtml
}

func buildAlertForm(alert *model.Alert) (string, error) {
	conn, err := database.Connect()

	if err != nil {
		return "", err
	}

	defer conn.Close()

	currencyList := make([]model.Currency, 0, 500)

	rows, err := conn.Query(currencyQuery + "order by name")

	if err != nil {
		return "", err
	}

	var currency model.Currency

	for rows.Next() {
		if err := ScanCurrency(rows, &currency); err != nil {
			return "", err
		}

		currencyList = append(currencyList, currency)
	}

	var alertFormTemplate = `
		<form method="post">
			<p>Price Alert</p>
			<p>Alert me when value of...</p>
			<select name="from">
				{fromOptions}
			</select>
			<p>goes</p>
			<label>
				<input type="radio" name="direction" value="above"{aboveChecked}>
				above
			</label>
			<label>
				<input type="radio" name="direction" value="below"{belowChecked}>
				below
			</label>
			<input type="text" pattern="^\d*(\.\d*)?$" name="value" value="{value}">
			<select name="to">
				{toOptions}
			</select>
			<button>Submit</button>
		</form>
	`

	fromOptions := buildCurrencyOptionsHtml(currencyList, alert.From.ID)
	toOptions := buildCurrencyOptionsHtml(currencyList, alert.To.ID)

	method := "post"
	aboveChecked := ""
	belowChecked := ""

	if alert.Above {
		aboveChecked = " checked"
	} else {
		belowChecked = " checked"
	}

	html := alertFormTemplate
	html = strings.Replace(html, "{method}", method, 1)
	html = strings.Replace(html, "{aboveChecked}", aboveChecked, 1)
	html = strings.Replace(html, "{belowChecked}", belowChecked, 1)
	html = strings.Replace(html, "{value}", alert.Value.String(), 1)
	html = strings.Replace(html, "{fromOptions}", fromOptions, 1)
	html = strings.Replace(html, "{toOptions}", toOptions, 1)

	return html, nil
}

func HandleAlertList(writer http.ResponseWriter, request *http.Request) {
	var alertListBodyTemplate = `
		{alertForm}
		<hr>
		<table>
			<tbody>
				{alertRows}
			</tbody>
		</table>
		<script>
			document
				.querySelectorAll("button[data-delete-id]")
				.forEach(button => {
					const id = button.dataset.deleteId

					button.addEventListener("click", () => {
						fetch("/alert/" + id, {
							method: "DELETE",
						})
							.then(response => {
								if (response.ok) {
									window.location.reload()
								}
							})
					})
				})
		</script>
	`
	var alertRowTemplate = `
		<tr>
			<td>{alertDescription}</td>
			<td><a href="/alert/{alertID}">Update</a></td>
			<td><button type="button" data-delete-id="{alertID}">Delete</a></td>
		</tr>
	`

	user := requireUser(writer, request, RedirectIfNoUser)

	if user == nil {
		return
	}

	alertList, err := loadAlertList(user.ID)

	if err != nil {
		util.RespondInternalServerError(writer, err)

		return
	}

	var alertRowsHtml string

	for _, alert := range alertList {
		direction := "≤"

		if alert.Above {
			direction = "≥"
		}

		alertDescription := html.EscapeString(fmt.Sprintf(
			"%s %s %s %s",
			alert.From.Name,
			direction,
			alert.Value.StringFixed(2),
			alert.To.Name,
		))

		html := strings.Replace(alertRowTemplate, "{alertDescription}", alertDescription, 1)
		html = strings.Replace(html, "{alertID}", strconv.Itoa(alert.ID), -1)

		alertRowsHtml += html
	}

	newAlert := model.Alert{Above: true}
	alertForm, err := buildAlertForm(&newAlert)

	if err != nil {
		util.RespondInternalServerError(writer, err)

		return
	}

	html := htmlTemplate
	html = strings.Replace(html, "{htmlBody}", alertListBodyTemplate, 1)
	html = strings.Replace(html, "{alertForm}", alertForm, 1)
	html = strings.Replace(html, "{alertRows}", alertRowsHtml, 1)
	fmt.Fprint(writer, html)
}

func loadAlertForRequest(writer http.ResponseWriter, request *http.Request) *model.Alert {
	user := requireUser(writer, request, RedirectIfNoUser)

	if user == nil {
		return nil
	}

	alertID, err := strconv.Atoi(mux.Vars(request)["id"])

	if err != nil {
		util.RespondNotFound(writer)

		return nil
	}

	conn, err := database.Connect()

	defer conn.Close()

	row := conn.QueryRow(alertQuery + " where user_id = $1 and crypto_alert.id = $2", user.ID, alertID)

	var alert model.Alert

	if err := ScanAlert(row, &alert); err != nil {
		if err == database.ErrNoRows {
			util.RespondNotFound(writer)
		} else {
			util.RespondInternalServerError(writer, err)
		}

		return nil
	}

	return &alert
}

func HandleAlert(writer http.ResponseWriter, request *http.Request) {
	var alertBodyTemplate = `
		{alertForm}
	`

	if alert := loadAlertForRequest(writer, request); alert != nil {
		alertForm, err := buildAlertForm(alert)

		if err != nil {
			util.RespondInternalServerError(writer, err)

			return
		}

		html := htmlTemplate
		html = strings.Replace(html, "{htmlBody}", alertBodyTemplate, 1)
		html = strings.Replace(html, "{alertForm}", alertForm, 1)
		fmt.Fprint(writer, html)
	}
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
	alert := loadAlertForRequest(writer, request)

	if alert == nil {
		return
	}

	if loadAlertFromRequest(alert, writer, request) {
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
	if alert := loadAlertForRequest(writer, request); alert != nil {
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
}
