// Notify users about prices going over a threshold
package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/dense-analysis/pricewarp/internal/database"
	"github.com/dense-analysis/pricewarp/internal/env"
	"github.com/shopspring/decimal"
)

type CryptoAlert struct {
	Id               int64
	UserID           int64
	Email            string
	FromCurrencyName string
	FromCurrencyTick string
	ToCurrencyName   string
	ToCurrencyTick   string
	Above            bool
	Value            decimal.Decimal
	AlertTime        time.Time
}

const (
	smtpDialTimeout      = 10 * time.Second
	smtpOperationTimeout = 30 * time.Second
)

func findAlertsToTrigger(conn *database.Conn) ([]*CryptoAlert, error) {
	rows, err := conn.Query(
		`
			SELECT
				alerts.alert_id,
				alerts.user_id,
				alerts.username,
				alerts.from_currency_name,
				alerts.from_currency_ticker,
				alerts.to_currency_name,
				alerts.to_currency_ticker,
				alerts.above,
				alerts.value,
				alerts.alert_time
			FROM (
				SELECT
					alert_id,
					user_id,
					username,
					from_currency_name,
					from_currency_ticker,
					to_currency_name,
					to_currency_ticker,
					above,
					value,
					alert_time,
					sent,
					is_deleted
				FROM crypto_alert
				ORDER BY updated_at DESC
				LIMIT 1 BY alert_id
			) AS alerts
			INNER JOIN (
				SELECT
					from_currency_ticker,
					to_currency_ticker,
					value,
					time
				FROM crypto_currency_prices
				ORDER BY time DESC
				LIMIT 1 BY from_currency_ticker, to_currency_ticker
			) AS prices
			ON prices.from_currency_ticker = alerts.from_currency_ticker
			AND prices.to_currency_ticker = alerts.to_currency_ticker
			WHERE alerts.is_deleted = 0
				AND alerts.sent = 0
				AND (
					(alerts.above = 1 AND prices.value >= alerts.value)
					OR (alerts.above = 0 AND prices.value <= alerts.value)
				)
				AND prices.time >= alerts.alert_time
		`,
	)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alertList []*CryptoAlert

	for rows.Next() {
		alert := &CryptoAlert{}
		var above uint8
		var value decimal.Decimal

		if err := rows.Scan(
			&alert.Id,
			&alert.UserID,
			&alert.Email,
			&alert.FromCurrencyName,
			&alert.FromCurrencyTick,
			&alert.ToCurrencyName,
			&alert.ToCurrencyTick,
			&above,
			&value,
			&alert.AlertTime,
		); err != nil {
			return nil, err
		}
		alert.Above = above == 1
		alert.Value = value
		alertList = append(alertList, alert)
	}

	return alertList, rows.Err()
}

func sendEmail(to string, message string) error {
	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	from := os.Getenv("SMTP_FROM")
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	tlsconfig := &tls.Config{ServerName: host}
	auth := smtp.PlainAuth("", username, password, host)
	dialer := &net.Dialer{Timeout: smtpDialTimeout}

	addr := host + ":" + port
	var client *smtp.Client
	var err error

	if port == "465" {
		var conn *tls.Conn
		if conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsconfig); err != nil {
			return err
		}

		if err := conn.SetDeadline(time.Now().Add(smtpOperationTimeout)); err != nil {
			_ = conn.Close()
			return err
		}

		defer func() {
			_ = conn.Close()
		}()

		if client, err = smtp.NewClient(conn, host); err != nil {
			return err
		}
	} else {
		var conn net.Conn
		if conn, err = dialer.Dial("tcp", addr); err != nil {
			return err
		}

		if err := conn.SetDeadline(time.Now().Add(smtpOperationTimeout)); err != nil {
			_ = conn.Close()
			return err
		}

		defer func() {
			_ = conn.Close()
		}()

		if client, err = smtp.NewClient(conn, host); err != nil {
			return err
		}
		if err = client.StartTLS(tlsconfig); err != nil {
			_ = client.Close()
			return err
		}
	}

	defer client.Close()

	if err = client.Auth(auth); err != nil {
		return err
	}

	if err = client.Mail(from); err != nil {
		return err
	}

	if err = client.Rcpt(to); err != nil {
		return err
	}

	var writer io.WriteCloser

	if writer, err = client.Data(); err != nil {
		return err
	}

	if _, err = writer.Write([]byte(message)); err != nil {
		return err
	}

	if err = writer.Close(); err != nil {
		return err
	}

	return client.Quit()
}

func sendAlertEmails(alertList []*CryptoAlert) error {
	from := os.Getenv("SMTP_FROM")
	groupedAlerts := map[string][]*CryptoAlert{}

	for _, alert := range alertList {
		groupedAlerts[alert.Email] = append(groupedAlerts[alert.Email], alert)
	}

	for email, groupedList := range groupedAlerts {
		message := `To: {to}
From: {from}
Subject: Price Alert
Content-Type: text/plain; charset=UTF-8; format=flowed
Content-Transfer-Encoding: 7bit

Prices have changed recently:

{priceString}
`
		priceStringLines := make([]string, len(groupedList))

		for i, alert := range groupedList {
			operator := "<="

			if alert.Above {
				operator = ">="
			}

			priceStringLines[i] = fmt.Sprintf(
				"1 %s %s %s %s",
				alert.FromCurrencyName,
				operator,
				alert.Value,
				alert.ToCurrencyName,
			)
		}

		message = strings.Replace(message, "{to}", email, -1)
		message = strings.Replace(message, "{from}", from, -1)
		message = strings.Replace(message, "{priceString}", strings.Join(priceStringLines, "\n"), -1)

		err := sendEmail(email, message)

		if err != nil {
			return err
		}
	}

	return nil
}

func markAlertsAsSent(conn *database.Conn, alertList []*CryptoAlert) error {
	batch, err := conn.PrepareBatch(
		`insert into crypto_alert
			(alert_id, user_id, username, above, alert_time, sent, value,
			 from_currency_ticker, from_currency_name,
			 to_currency_ticker, to_currency_name,
			 updated_at, is_deleted)
		values (?, ?, ?, ?, ?, 1, ?,
			?, ?,
			?, ?,
			now64(9), 0)`,
	)

	if err != nil {
		return err
	}

	for _, alert := range alertList {
		if err := batch.Append(
			alert.Id,
			alert.UserID,
			alert.Email,
			boolToUint(alert.Above),
			alert.AlertTime,
			alert.Value,
			alert.FromCurrencyTick,
			alert.FromCurrencyName,
			alert.ToCurrencyTick,
			alert.ToCurrencyName,
		); err != nil {
			return err
		}
	}

	return batch.Send()
}

func main() {
	env.LoadEnvironmentVariables()

	conn, err := database.Connect()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Connection error: %s\n", err)
		os.Exit(1)
	}

	defer conn.Close()

	alertList, err := findAlertsToTrigger(conn)

	if err != nil {
		fmt.Fprintf(os.Stderr, "SQL error: %s\n", err)
		os.Exit(1)
	}

	err = sendAlertEmails(alertList)

	if err != nil {
		fmt.Fprintf(os.Stderr, "SMTP error: %s\n", err)
		os.Exit(1)
	}

	if len(alertList) > 0 {
		err = markAlertsAsSent(conn, alertList)

		if err != nil {
			fmt.Fprintf(os.Stderr, "SQL error: %s\n", err)
			os.Exit(1)
		}
	}
}

func boolToUint(value bool) uint8 {
	if value {
		return 1
	}

	return 0
}
