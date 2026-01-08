#!/usr/bin/env bash

set -euo pipefail

# shellcheck disable=SC2046
export $(xargs < .env)

EXPORT_DIR=${1:-export}

clickhouse-client --host "$DB_HOST" --port "$DB_PORT" --user "$DB_USERNAME" --password "$DB_PASSWORD" --database "$DB_NAME" \
  --query="INSERT INTO crypto_user_login (user_id, username, password_hash, created_at, updated_at, is_active) FORMAT CSVWithNames" \
  < "${EXPORT_DIR}/crypto_user_login.csv"

clickhouse-client --host "$DB_HOST" --port "$DB_PORT" --user "$DB_USERNAME" --password "$DB_PASSWORD" --database "$DB_NAME" \
  --query="INSERT INTO crypto_currency (id, ticker, name, updated_at) FORMAT CSVWithNames" \
  < "${EXPORT_DIR}/crypto_currency.csv"

clickhouse-client --host "$DB_HOST" --port "$DB_PORT" --user "$DB_USERNAME" --password "$DB_PASSWORD" --database "$DB_NAME" \
  --query="INSERT INTO crypto_currency_prices (time, from_currency_id, from_currency_ticker, from_currency_name, to_currency_id, to_currency_ticker, to_currency_name, value) FORMAT CSVWithNames" \
  < "${EXPORT_DIR}/crypto_currency_prices.csv"

clickhouse-client --host "$DB_HOST" --port "$DB_PORT" --user "$DB_USERNAME" --password "$DB_PASSWORD" --database "$DB_NAME" \
  --query="INSERT INTO crypto_alert (alert_id, user_id, username, from_currency_id, from_currency_ticker, from_currency_name, to_currency_id, to_currency_ticker, to_currency_name, value, above, alert_time, sent, updated_at, is_deleted) FORMAT CSVWithNames" \
  < "${EXPORT_DIR}/crypto_alert.csv"

clickhouse-client --host "$DB_HOST" --port "$DB_PORT" --user "$DB_USERNAME" --password "$DB_PASSWORD" --database "$DB_NAME" \
  --query="INSERT INTO crypto_portfolio (user_id, username, currency_id, currency_ticker, currency_name, cash, updated_at, is_deleted) FORMAT CSVWithNames" \
  < "${EXPORT_DIR}/crypto_portfolio.csv"

clickhouse-client --host "$DB_HOST" --port "$DB_PORT" --user "$DB_USERNAME" --password "$DB_PASSWORD" --database "$DB_NAME" \
  --query="INSERT INTO crypto_asset (user_id, username, currency_id, currency_ticker, currency_name, purchased, amount, updated_at, is_deleted) FORMAT CSVWithNames" \
  < "${EXPORT_DIR}/crypto_asset.csv"
