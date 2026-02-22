#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 2 ]; then
    echo "Usage: $0 PRICES_CSV CURRENCIES_CSV" >&2
    exit 1
fi

PRICES_CSV="$1"
CURRENCIES_CSV="$2"

if [ ! -f "$PRICES_CSV" ]; then
    echo "Prices CSV not found: $PRICES_CSV" >&2
    exit 1
fi

if [ ! -f "$CURRENCIES_CSV" ]; then
    echo "Currencies CSV not found: $CURRENCIES_CSV" >&2
    exit 1
fi

# shellcheck disable=SC2046
export $(xargs < .env)

if ! command -v clickhouse-client >/dev/null 2>&1; then
    clickhouse-client() {
        ~/clickhouse/clickhouse client "$@"
    }
fi

clickhouse-client \
    --host "$DB_HOST" \
    --port "$DB_PORT" \
    --user "$DB_USERNAME" \
    --password "$DB_PASSWORD" \
    --database "$DB_NAME" \
    --query "
        CREATE TABLE IF NOT EXISTS crypto_currency_prices_backfill_staging
        (
            time DateTime64(9),
            from_currency_ticker LowCardinality(String),
            from_currency_name LowCardinality(String),
            to_currency_ticker LowCardinality(String),
            to_currency_name LowCardinality(String),
            value Decimal(40, 20)
        )
        ENGINE = MergeTree
        ORDER BY (time, from_currency_ticker, to_currency_ticker)
    "

clickhouse-client \
    --host "$DB_HOST" \
    --port "$DB_PORT" \
    --user "$DB_USERNAME" \
    --password "$DB_PASSWORD" \
    --database "$DB_NAME" \
    --query "
        CREATE TABLE IF NOT EXISTS crypto_currencies_backfill_staging
        (
            ticker LowCardinality(String),
            name LowCardinality(String),
            updated_at DateTime64(9)
        )
        ENGINE = MergeTree
        ORDER BY ticker
    "

clickhouse-client \
    --host "$DB_HOST" \
    --port "$DB_PORT" \
    --user "$DB_USERNAME" \
    --password "$DB_PASSWORD" \
    --database "$DB_NAME" \
    --query "TRUNCATE TABLE crypto_currency_prices_backfill_staging"

clickhouse-client \
    --host "$DB_HOST" \
    --port "$DB_PORT" \
    --user "$DB_USERNAME" \
    --password "$DB_PASSWORD" \
    --database "$DB_NAME" \
    --query "TRUNCATE TABLE crypto_currencies_backfill_staging"

clickhouse-client \
    --host "$DB_HOST" \
    --port "$DB_PORT" \
    --user "$DB_USERNAME" \
    --password "$DB_PASSWORD" \
    --database "$DB_NAME" \
    --query "
        INSERT INTO crypto_currencies_backfill_staging
        (ticker, name, updated_at)
        FORMAT CSVWithNames
    " < "$CURRENCIES_CSV"

clickhouse-client \
    --host "$DB_HOST" \
    --port "$DB_PORT" \
    --user "$DB_USERNAME" \
    --password "$DB_PASSWORD" \
    --database "$DB_NAME" \
    --query "
        INSERT INTO crypto_currency_prices_backfill_staging
        (time, from_currency_ticker, from_currency_name, to_currency_ticker, to_currency_name, value)
        FORMAT CSVWithNames
    " < "$PRICES_CSV"

clickhouse-client \
    --host "$DB_HOST" \
    --port "$DB_PORT" \
    --user "$DB_USERNAME" \
    --password "$DB_PASSWORD" \
    --database "$DB_NAME" \
    --query "
        INSERT INTO crypto_currencies (ticker, name, updated_at)
        SELECT
            staged.ticker,
            staged.name,
            now64(9)
        FROM crypto_currencies_backfill_staging AS staged
        LEFT JOIN
        (
            SELECT ticker FROM crypto_currencies GROUP BY ticker
        ) AS current
        ON staged.ticker = current.ticker
        WHERE current.ticker IS NULL
    "

clickhouse-client \
    --host "$DB_HOST" \
    --port "$DB_PORT" \
    --user "$DB_USERNAME" \
    --password "$DB_PASSWORD" \
    --database "$DB_NAME" \
    --query "
        INSERT INTO crypto_currency_prices
            (time, from_currency_ticker, from_currency_name, to_currency_ticker, to_currency_name, value)
        SELECT
            staging.time,
            staging.from_currency_ticker,
            staging.from_currency_name,
            staging.to_currency_ticker,
            staging.to_currency_name,
            staging.value
        FROM crypto_currency_prices_backfill_staging AS staging
        LEFT JOIN
        (
            SELECT
                time,
                from_currency_ticker,
                to_currency_ticker
            FROM crypto_currency_prices
            GROUP BY time, from_currency_ticker, to_currency_ticker
        ) AS existing
        ON
            staging.time = existing.time
            AND staging.from_currency_ticker = existing.from_currency_ticker
            AND staging.to_currency_ticker = existing.to_currency_ticker
        WHERE existing.time IS NULL
    "

clickhouse-client \
    --host "$DB_HOST" \
    --port "$DB_PORT" \
    --user "$DB_USERNAME" \
    --password "$DB_PASSWORD" \
    --database "$DB_NAME" \
    --query "TRUNCATE TABLE crypto_currency_prices_backfill_staging"

clickhouse-client \
    --host "$DB_HOST" \
    --port "$DB_PORT" \
    --user "$DB_USERNAME" \
    --password "$DB_PASSWORD" \
    --database "$DB_NAME" \
    --query "TRUNCATE TABLE crypto_currencies_backfill_staging"

echo "Loaded backfill CSV into ClickHouse."
