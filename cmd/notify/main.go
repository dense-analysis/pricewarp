// Notify users about prices going over a threshold
package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/smtp"
	"os"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/dense-analysis/pricewarp/internal/database"
	"github.com/dense-analysis/pricewarp/internal/env"
)

type CryptoAlert struct {
	Id               int
	Email            string
	FromCurrencyName string
	ToCurrencyName   string
	Above            bool
	Value            decimal.Decimal
}

func findAlertsToTrigger(conn *database.Conn) ([]*CryptoAlert, error) {
	rows, err := conn.Query(
		`
			SELECT
				crypto_alert.id,
				crypto_user.username,
				from_currency.name,
				to_currency.name,
				above,
				value
			FROM crypto_alert
			INNER JOIN crypto_user
			ON crypto_user.id = crypto_alert.user_id
			INNER JOIN crypto_currency AS from_currency
			ON from_currency.id = crypto_alert."from"
			INNER JOIN crypto_currency AS to_currency
			ON to_currency.id = crypto_alert."to"
			WHERE NOT sent AND EXISTS (
				SELECT FROM crypto_price
				WHERE crypto_price."from" = crypto_alert."from"
				AND crypto_price."to" = crypto_alert."to"
				AND (
					(crypto_alert."above" AND crypto_price.value >= crypto_alert."value")
					OR (NOT crypto_alert."above" AND crypto_price.value <= crypto_alert."value")
				)
				AND crypto_price.time >= crypto_alert.time
			)
		`,
	)

	if err != nil {
		return nil, err
	}

	var alertList []*CryptoAlert

	for rows.Next() {
		alert := &CryptoAlert{}
		rows.Scan(
			&alert.Id,
			&alert.Email,
			&alert.FromCurrencyName,
			&alert.ToCurrencyName,
			&alert.Above,
			&alert.Value,
		)
		alertList = append(alertList, alert)
	}

	return alertList, nil
}

func sendEmail(to string, message string) error {
	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	from := os.Getenv("SMTP_FROM")
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	tlsconfig := &tls.Config{ServerName: host}
	auth := smtp.PlainAuth("", username, password, host)

	var conn *tls.Conn
	var err error

	if conn, err = tls.Dial("tcp", host+":"+port, tlsconfig); err != nil {
		return err
	}

	defer conn.Close()

	var client *smtp.Client

	if client, err = smtp.NewClient(conn, host); err != nil {
		return err
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
	markers := make([]string, len(alertList))
	ids := make([]interface{}, len(alertList))

	for i, alert := range alertList {
		markers[i] = "$" + strconv.Itoa(i+1)
		ids[i] = alert.Id
	}

	query := "UPDATE crypto_alert SET sent = true WHERE id IN (" + strings.Join(markers, ",") + ")"

	return conn.Exec(query, ids...)
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
