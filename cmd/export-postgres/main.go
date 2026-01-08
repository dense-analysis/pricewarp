// Export Postgres tables into CSV files for ClickHouse imports.
package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dense-analysis/pricewarp/internal/env"
	"github.com/jackc/pgx/v4"
)

type exportConfig struct {
	outputDir  string
	createdAt  time.Time
	updatedAt  time.Time
	pgHost     string
	pgPort     string
	pgUser     string
	pgPassword string
	pgDatabase string
}

func main() {
	env.LoadEnvironmentVariables()

	config := buildConfig()

	conn, err := pgx.Connect(
		context.Background(),
		fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s",
			config.pgUser,
			config.pgPassword,
			config.pgHost,
			config.pgPort,
			config.pgDatabase,
		),
	)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Connection error: %s\n", err)
		os.Exit(1)
	}

	defer conn.Close(context.Background())

	if err := os.MkdirAll(config.outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %s\n", err)
		os.Exit(1)
	}

	if err := exportUsers(conn, config); err != nil {
		exitWithError("Export users", err)
	}

	if err := exportCurrencies(conn, config); err != nil {
		exitWithError("Export currencies", err)
	}

	if err := exportPrices(conn, config); err != nil {
		exitWithError("Export prices", err)
	}

	if err := exportAlerts(conn, config); err != nil {
		exitWithError("Export alerts", err)
	}

	if err := exportPortfolios(conn, config); err != nil {
		exitWithError("Export portfolios", err)
	}

	if err := exportAssets(conn, config); err != nil {
		exitWithError("Export assets", err)
	}
}

func buildConfig() exportConfig {
	now := time.Now().UTC()

	return exportConfig{
		outputDir:  argOrDefault(1, "export"),
		createdAt:  now,
		updatedAt:  now,
		pgHost:     envOrDefault("PG_HOST", os.Getenv("DB_HOST")),
		pgPort:     envOrDefault("PG_PORT", os.Getenv("DB_PORT")),
		pgUser:     envOrDefault("PG_USERNAME", os.Getenv("DB_USERNAME")),
		pgPassword: envOrDefault("PG_PASSWORD", os.Getenv("DB_PASSWORD")),
		pgDatabase: envOrDefault("PG_DATABASE", os.Getenv("DB_NAME")),
	}
}

func exportUsers(conn *pgx.Conn, config exportConfig) error {
	rows, err := conn.Query(context.Background(), "select id, username, password from crypto_user order by id")

	if err != nil {
		return err
	}

	defer rows.Close()

	path := filepath.Join(config.outputDir, "crypto_user_login.csv")
	writer, file, err := createCSV(path)

	if err != nil {
		return err
	}

	defer file.Close()

	if err := writer.Write([]string{
		"user_id",
		"username",
		"password_hash",
		"created_at",
		"updated_at",
		"is_active",
	}); err != nil {
		return err
	}

	createdAt := formatTime(config.createdAt)
	updatedAt := formatTime(config.updatedAt)

	for rows.Next() {
		var id int64
		var username string
		var password string

		if err := rows.Scan(&id, &username, &password); err != nil {
			return err
		}

		if err := writer.Write([]string{
			fmt.Sprintf("%d", id),
			username,
			password,
			createdAt,
			updatedAt,
			"1",
		}); err != nil {
			return err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}

	return rows.Err()
}

func exportCurrencies(conn *pgx.Conn, config exportConfig) error {
	rows, err := conn.Query(context.Background(), "select id, ticker, name from crypto_currency order by id")

	if err != nil {
		return err
	}

	defer rows.Close()

	path := filepath.Join(config.outputDir, "crypto_currency.csv")
	writer, file, err := createCSV(path)

	if err != nil {
		return err
	}

	defer file.Close()

	if err := writer.Write([]string{"id", "ticker", "name", "updated_at"}); err != nil {
		return err
	}

	updatedAt := formatTime(config.updatedAt)

	for rows.Next() {
		var id int64
		var ticker string
		var name string

		if err := rows.Scan(&id, &ticker, &name); err != nil {
			return err
		}

		if err := writer.Write([]string{
			fmt.Sprintf("%d", id),
			ticker,
			name,
			updatedAt,
		}); err != nil {
			return err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}

	return rows.Err()
}

func exportPrices(conn *pgx.Conn, config exportConfig) error {
	rows, err := conn.Query(
		context.Background(),
		`
			select
				price.time,
				from_currency.id,
				from_currency.ticker,
				from_currency.name,
				to_currency.id,
				to_currency.ticker,
				to_currency.name,
				price.value
			from crypto_price as price
			inner join crypto_currency as from_currency
				on from_currency.id = price."from"
			inner join crypto_currency as to_currency
				on to_currency.id = price."to"
			order by price.time
		`,
	)

	if err != nil {
		return err
	}

	defer rows.Close()

	path := filepath.Join(config.outputDir, "crypto_currency_prices.csv")
	writer, file, err := createCSV(path)

	if err != nil {
		return err
	}

	defer file.Close()

	if err := writer.Write([]string{
		"time",
		"from_currency_id",
		"from_currency_ticker",
		"from_currency_name",
		"to_currency_id",
		"to_currency_ticker",
		"to_currency_name",
		"value",
	}); err != nil {
		return err
	}

	for rows.Next() {
		var timestamp time.Time
		var fromID int64
		var fromTicker string
		var fromName string
		var toID int64
		var toTicker string
		var toName string
		var value string

		if err := rows.Scan(
			&timestamp,
			&fromID,
			&fromTicker,
			&fromName,
			&toID,
			&toTicker,
			&toName,
			&value,
		); err != nil {
			return err
		}

		if err := writer.Write([]string{
			formatTime(timestamp),
			fmt.Sprintf("%d", fromID),
			fromTicker,
			fromName,
			fmt.Sprintf("%d", toID),
			toTicker,
			toName,
			value,
		}); err != nil {
			return err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}

	return rows.Err()
}

func exportAlerts(conn *pgx.Conn, config exportConfig) error {
	rows, err := conn.Query(
		context.Background(),
		`
			select
				alert.id,
				alert.user_id,
				crypto_user.username,
				from_currency.id,
				from_currency.ticker,
				from_currency.name,
				to_currency.id,
				to_currency.ticker,
				to_currency.name,
				alert.value,
				alert.above,
				alert.time,
				alert.sent
			from crypto_alert as alert
			inner join crypto_user
				on crypto_user.id = alert.user_id
			inner join crypto_currency as from_currency
				on from_currency.id = alert."from"
			inner join crypto_currency as to_currency
				on to_currency.id = alert."to"
			order by alert.time
		`,
	)

	if err != nil {
		return err
	}

	defer rows.Close()

	path := filepath.Join(config.outputDir, "crypto_alert.csv")
	writer, file, err := createCSV(path)

	if err != nil {
		return err
	}

	defer file.Close()

	if err := writer.Write([]string{
		"alert_id",
		"user_id",
		"username",
		"from_currency_id",
		"from_currency_ticker",
		"from_currency_name",
		"to_currency_id",
		"to_currency_ticker",
		"to_currency_name",
		"value",
		"above",
		"alert_time",
		"sent",
		"updated_at",
		"is_deleted",
	}); err != nil {
		return err
	}

	for rows.Next() {
		var alertID int64
		var userID int64
		var username string
		var fromID int64
		var fromTicker string
		var fromName string
		var toID int64
		var toTicker string
		var toName string
		var value string
		var above bool
		var alertTime time.Time
		var sent bool

		if err := rows.Scan(
			&alertID,
			&userID,
			&username,
			&fromID,
			&fromTicker,
			&fromName,
			&toID,
			&toTicker,
			&toName,
			&value,
			&above,
			&alertTime,
			&sent,
		); err != nil {
			return err
		}

		if err := writer.Write([]string{
			fmt.Sprintf("%d", alertID),
			fmt.Sprintf("%d", userID),
			username,
			fmt.Sprintf("%d", fromID),
			fromTicker,
			fromName,
			fmt.Sprintf("%d", toID),
			toTicker,
			toName,
			value,
			boolToUInt8(above),
			formatTime(alertTime),
			boolToUInt8(sent),
			formatTime(alertTime),
			"0",
		}); err != nil {
			return err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}

	return rows.Err()
}

func exportPortfolios(conn *pgx.Conn, config exportConfig) error {
	rows, err := conn.Query(
		context.Background(),
		`
			select
				portfolio.user_id,
				crypto_user.username,
				portfolio.currency_id,
				currency.ticker,
				currency.name,
				portfolio.cash
			from crypto_portfolio as portfolio
			inner join crypto_user
				on crypto_user.id = portfolio.user_id
			inner join crypto_currency as currency
				on currency.id = portfolio.currency_id
			order by portfolio.user_id
		`,
	)

	if err != nil {
		return err
	}

	defer rows.Close()

	path := filepath.Join(config.outputDir, "crypto_portfolio.csv")
	writer, file, err := createCSV(path)

	if err != nil {
		return err
	}

	defer file.Close()

	if err := writer.Write([]string{
		"user_id",
		"username",
		"currency_id",
		"currency_ticker",
		"currency_name",
		"cash",
		"updated_at",
		"is_deleted",
	}); err != nil {
		return err
	}

	updatedAt := formatTime(config.updatedAt)

	for rows.Next() {
		var userID int64
		var username string
		var currencyID int64
		var ticker string
		var name string
		var cash string

		if err := rows.Scan(&userID, &username, &currencyID, &ticker, &name, &cash); err != nil {
			return err
		}

		if err := writer.Write([]string{
			fmt.Sprintf("%d", userID),
			username,
			fmt.Sprintf("%d", currencyID),
			ticker,
			name,
			cash,
			updatedAt,
			"0",
		}); err != nil {
			return err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}

	return rows.Err()
}

func exportAssets(conn *pgx.Conn, config exportConfig) error {
	rows, err := conn.Query(
		context.Background(),
		`
			select
				asset.user_id,
				crypto_user.username,
				asset.currency_id,
				currency.ticker,
				currency.name,
				asset.purchased,
				asset.amount
			from crypto_asset as asset
			inner join crypto_user
				on crypto_user.id = asset.user_id
			inner join crypto_currency as currency
				on currency.id = asset.currency_id
			order by asset.user_id, asset.currency_id
		`,
	)

	if err != nil {
		return err
	}

	defer rows.Close()

	path := filepath.Join(config.outputDir, "crypto_asset.csv")
	writer, file, err := createCSV(path)

	if err != nil {
		return err
	}

	defer file.Close()

	if err := writer.Write([]string{
		"user_id",
		"username",
		"currency_id",
		"currency_ticker",
		"currency_name",
		"purchased",
		"amount",
		"updated_at",
		"is_deleted",
	}); err != nil {
		return err
	}

	updatedAt := formatTime(config.updatedAt)

	for rows.Next() {
		var userID int64
		var username string
		var currencyID int64
		var ticker string
		var name string
		var purchased string
		var amount string

		if err := rows.Scan(
			&userID,
			&username,
			&currencyID,
			&ticker,
			&name,
			&purchased,
			&amount,
		); err != nil {
			return err
		}

		if err := writer.Write([]string{
			fmt.Sprintf("%d", userID),
			username,
			fmt.Sprintf("%d", currencyID),
			ticker,
			name,
			purchased,
			amount,
			updatedAt,
			"0",
		}); err != nil {
			return err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}

	return rows.Err()
}

func createCSV(path string) (*csv.Writer, *os.File, error) {
	file, err := os.Create(path)

	if err != nil {
		return nil, nil, err
	}

	writer := csv.NewWriter(file)

	return writer, file, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func boolToUInt8(value bool) string {
	if value {
		return "1"
	}

	return "0"
}

func argOrDefault(position int, fallback string) string {
	if len(os.Args) > position {
		return os.Args[position]
	}

	return fallback
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}

func exitWithError(action string, err error) {
	fmt.Fprintf(os.Stderr, "%s error: %s\n", action, err)
	os.Exit(1)
}
