#!/usr/bin/env bash

set -euo pipefail

# shellcheck disable=SC2046
export $(xargs < .env)

if ! command -v clickhouse-client &> /dev/null; then
    clickhouse-client() {
        ~/clickhouse/clickhouse client "$@"
    }
fi

clickhouse-client --host "$DB_HOST" --port "$DB_PORT" --user "$DB_USERNAME" --password "$DB_PASSWORD" --database "$DB_NAME" \
  --query="INSERT INTO crypto_users (user_id, username, password_hash, created_at, updated_at, is_active) FORMAT CSVWithNames" \
  < crypto_users.csv

clickhouse-client --host "$DB_HOST" --port "$DB_PORT" --user "$DB_USERNAME" --password "$DB_PASSWORD" --database "$DB_NAME" \
  --query="INSERT INTO crypto_currencies (ticker, name, updated_at) FORMAT CSVWithNames" \
  < crypto_currencies.csv

clickhouse-client --host "$DB_HOST" --port "$DB_PORT" --user "$DB_USERNAME" --password "$DB_PASSWORD" --database "$DB_NAME" \
  --query="INSERT INTO crypto_currency_prices (time, from_currency_ticker, from_currency_name, to_currency_ticker, to_currency_name, value) FORMAT CSVWithNames" \
  < crypto_currency_prices.csv

clickhouse-client --host "$DB_HOST" --port "$DB_PORT" --user "$DB_USERNAME" --password "$DB_PASSWORD" --database "$DB_NAME" \
  --query="INSERT INTO crypto_alert (alert_id, user_id, username, from_currency_ticker, from_currency_name, to_currency_ticker, to_currency_name, value, above, alert_time, sent, updated_at, is_deleted) FORMAT CSVWithNames" \
  < crypto_alert.csv

clickhouse-client --host "$DB_HOST" --port "$DB_PORT" --user "$DB_USERNAME" --password "$DB_PASSWORD" --database "$DB_NAME" \
  --query="INSERT INTO crypto_portfolio (user_id, username, currency_ticker, currency_name, cash, updated_at, is_deleted) FORMAT CSVWithNames" \
  < crypto_portfolio.csv

clickhouse-client --host "$DB_HOST" --port "$DB_PORT" --user "$DB_USERNAME" --password "$DB_PASSWORD" --database "$DB_NAME" \
  --query="INSERT INTO crypto_asset (user_id, username, currency_ticker, currency_name, purchased, amount, updated_at, is_deleted) FORMAT CSVWithNames" \
  < crypto_asset.csv
