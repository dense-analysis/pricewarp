#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
    echo "Usage: $0 PRICES_CSV [CURRENCIES_CSV]" >&2
    exit 1
fi

PRICES_CSV="$1"
CURRENCIES_CSV="${2:-}"

if [ ! -f "$PRICES_CSV" ]; then
    echo "Prices CSV not found: $PRICES_CSV" >&2
    exit 1
fi

if [ -n "$CURRENCIES_CSV" ] && [ ! -f "$CURRENCIES_CSV" ]; then
    echo "Currencies CSV not found: $CURRENCIES_CSV" >&2
    exit 1
fi

if [ ! -f ".env" ]; then
    echo "Missing .env file in repository root" >&2
    exit 1
fi

DB_HOST="${DB_HOST:-}"
DB_PORT="${DB_PORT:-}"
DB_USERNAME="${DB_USERNAME:-}"
DB_PASSWORD="${DB_PASSWORD:-}"
DB_NAME="${DB_NAME:-}"

while IFS= read -r raw_line || [ -n "$raw_line" ]; do
    line="${raw_line%$'\r'}"

    case "$line" in
        ''|\#*)
            continue
            ;;
    esac

    if [[ "$line" != *=* ]]; then
        continue
    fi

    key="${line%%=*}"
    value="${line#*=}"

    case "$key" in
        DB_HOST)
            DB_HOST="$value"
            ;;
        DB_PORT)
            DB_PORT="$value"
            ;;
        DB_USERNAME)
            DB_USERNAME="$value"
            ;;
        DB_PASSWORD)
            DB_PASSWORD="$value"
            ;;
        DB_NAME)
            DB_NAME="$value"
            ;;
    esac
done < .env

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-9000}"
DB_USERNAME="${DB_USERNAME:-default}"
DB_NAME="${DB_NAME:-default}"

CLICKHOUSE_CLIENT_BIN="$(command -v clickhouse-client || true)"
CLICKHOUSE_BASE=("")

if [ -n "$CLICKHOUSE_CLIENT_BIN" ]; then
    CLICKHOUSE_BASE=("$CLICKHOUSE_CLIENT_BIN")
elif [ -x "$HOME/clickhouse/clickhouse" ]; then
    CLICKHOUSE_BASE=("$HOME/clickhouse/clickhouse" "client")
else
    echo "Could not find clickhouse client binary" >&2
    echo "Install clickhouse-client or ~/clickhouse/clickhouse" >&2
    exit 1
fi

ch() {
    "${CLICKHOUSE_BASE[@]}" \
        --host "$DB_HOST" \
        --port "$DB_PORT" \
        --user "$DB_USERNAME" \
        --password "$DB_PASSWORD" \
        --database "$DB_NAME" \
        "$@"
}

query() {
    local sql="$1"
    ch --query "$sql"
}

query_with_csv_input() {
    local sql="$1"
    local input_file="$2"
    ch --query "$sql" < "$input_file"
}

to_int() {
    local value="$1"
    value="${value//$'\n'/}"
    value="${value//$'\r'/}"
    value="${value//$' '/}"
    printf '%s' "$value"
}

echo "Creating staging tables..."
query "
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

query "
    CREATE TABLE IF NOT EXISTS crypto_currencies_backfill_staging
    (
        ticker LowCardinality(String),
        name LowCardinality(String),
        updated_at DateTime64(9)
    )
    ENGINE = MergeTree
    ORDER BY ticker
"

echo "Clearing staging tables..."
query "TRUNCATE TABLE crypto_currency_prices_backfill_staging"
query "TRUNCATE TABLE crypto_currencies_backfill_staging"

echo "Loading prices CSV into staging..."
query_with_csv_input "
    INSERT INTO crypto_currency_prices_backfill_staging
    (time, from_currency_ticker, from_currency_name, to_currency_ticker, to_currency_name, value)
    FORMAT CSVWithNames
" "$PRICES_CSV"

staging_price_rows_raw="$(query "SELECT count() FROM crypto_currency_prices_backfill_staging")"
staging_price_rows="$(to_int "$staging_price_rows_raw")"

if [ "$staging_price_rows" -eq 0 ]; then
    echo "No price rows were loaded from CSV." >&2
    exit 1
fi

if [ -n "$CURRENCIES_CSV" ]; then
    echo "Loading currencies CSV into staging..."
    query_with_csv_input "
        INSERT INTO crypto_currencies_backfill_staging
        (ticker, name, updated_at)
        FORMAT CSVWithNames
    " "$CURRENCIES_CSV"
else
    echo "Building currencies staging rows from prices CSV..."
    query "
        INSERT INTO crypto_currencies_backfill_staging (ticker, name, updated_at)
        SELECT
            src.ticker,
            src.ticker,
            now64(9)
        FROM
        (
            SELECT from_currency_ticker AS ticker FROM crypto_currency_prices_backfill_staging
            UNION DISTINCT
            SELECT to_currency_ticker AS ticker FROM crypto_currency_prices_backfill_staging
        ) AS src
    "
fi

before_currency_count_raw="$(query "SELECT count() FROM crypto_currencies")"
before_currency_count="$(to_int "$before_currency_count_raw")"

echo "Inserting missing currencies..."
query "
    INSERT INTO crypto_currencies (ticker, name, updated_at)
    SELECT
        staged.ticker,
        staged.name,
        now64(9)
    FROM
    (
        SELECT ticker, any(name) AS name
        FROM crypto_currencies_backfill_staging
        GROUP BY ticker
    ) AS staged
    WHERE staged.ticker NOT IN
    (
        SELECT ticker FROM crypto_currencies GROUP BY ticker
    )
"

after_currency_count_raw="$(query "SELECT count() FROM crypto_currencies")"
after_currency_count="$(to_int "$after_currency_count_raw")"

before_price_count_raw="$(query "
    SELECT count()
    FROM crypto_currency_prices
    WHERE time >= (SELECT min(time) FROM crypto_currency_prices_backfill_staging)
      AND time <= (SELECT max(time) FROM crypto_currency_prices_backfill_staging)
")"
before_price_count="$(to_int "$before_price_count_raw")"

echo "Inserting missing prices..."
query "
    INSERT INTO crypto_currency_prices
        (time, from_currency_ticker, from_currency_name, to_currency_ticker, to_currency_name, value)
    SELECT
        staged.time,
        staged.from_currency_ticker,
        staged.from_currency_name,
        staged.to_currency_ticker,
        staged.to_currency_name,
        staged.value
    FROM
    (
        SELECT
            time,
            from_currency_ticker,
            any(from_currency_name) AS from_currency_name,
            to_currency_ticker,
            any(to_currency_name) AS to_currency_name,
            max(value) AS value
        FROM crypto_currency_prices_backfill_staging
        GROUP BY time, from_currency_ticker, to_currency_ticker
    ) AS staged
    WHERE
        (staged.time, staged.from_currency_ticker, staged.to_currency_ticker)
        NOT IN
        (
            SELECT
                time,
                from_currency_ticker,
                to_currency_ticker
            FROM crypto_currency_prices
            WHERE time >= (SELECT min(time) FROM crypto_currency_prices_backfill_staging)
              AND time <= (SELECT max(time) FROM crypto_currency_prices_backfill_staging)
            GROUP BY time, from_currency_ticker, to_currency_ticker
        )
"

after_price_count_raw="$(query "
    SELECT count()
    FROM crypto_currency_prices
    WHERE time >= (SELECT min(time) FROM crypto_currency_prices_backfill_staging)
      AND time <= (SELECT max(time) FROM crypto_currency_prices_backfill_staging)
")"
after_price_count="$(to_int "$after_price_count_raw")"

query "TRUNCATE TABLE crypto_currency_prices_backfill_staging"
query "TRUNCATE TABLE crypto_currencies_backfill_staging"

currency_inserted=$((after_currency_count - before_currency_count))
price_inserted=$((after_price_count - before_price_count))

echo
echo "Load complete"
echo "Prices staging rows: $staging_price_rows"
echo "Currencies inserted: $currency_inserted"
echo "Prices in range before: $before_price_count"
echo "Prices in range after: $after_price_count"
echo "Prices inserted in range: $price_inserted"
