#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
    echo "Usage: $0 START_DATE END_DATE [OUTPUT_DIR]" >&2
    echo "Example: $0 2026-01-28 2026-02-21 ./data/backfill" >&2
    exit 1
fi

START_DATE="$1"
END_DATE="$2"
OUTPUT_DIR="${3:-./data/backfill}"
REQUEST_DELAY_SECONDS="${REQUEST_DELAY_SECONDS:-0.05}"

for dependency in curl jq python3; do
    dependency_path="$(command -v "$dependency" || true)"
    if [ -z "$dependency_path" ]; then
        echo "Missing dependency: $dependency" >&2
        exit 1
    fi
done

python3 - "$START_DATE" "$END_DATE" <<'PY'
import datetime as dt
import sys

start = dt.date.fromisoformat(sys.argv[1])
end = dt.date.fromisoformat(sys.argv[2])

if end < start:
    raise SystemExit("END_DATE must be on or after START_DATE")
PY

mkdir -p "$OUTPUT_DIR"

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

EXCHANGE_INFO_JSON="$WORK_DIR/exchange-info.json"
SYMBOLS_TSV="$WORK_DIR/symbols.tsv"
RAW_PRICES_FILE="$WORK_DIR/raw_prices.csv"

download_to_file() {
    local url="$1"
    local output_path="$2"
    local status_code=""

    echo "Grabbing: $url"

    status_code="$(curl \
        --silent \
        --show-error \
        --location \
        --connect-timeout 10 \
        --max-time 45 \
        --retry 2 \
        --retry-delay 1 \
        --output "$output_path" \
        --write-out "%{http_code}" \
        "$url")"

    if [ "$status_code" != "200" ]; then
        echo "HTTP $status_code: $url" >&2
        rm -f "$output_path"
        return 1
    fi

    if [ ! -s "$output_path" ]; then
        echo "Empty response body: $url" >&2
        rm -f "$output_path"
        return 1
    fi

    echo "Saved: $output_path"

    return 0
}

echo "Fetching Binance exchange metadata..."
if ! download_to_file "https://api.binance.com/api/v3/exchangeInfo" "$EXCHANGE_INFO_JSON"; then
    echo "Failed to fetch exchange metadata" >&2
    exit 1
fi

jq -r '
    .symbols[]
    | select(.status == "TRADING")
    | select(.isSpotTradingAllowed == true)
    | select(.quoteAsset as $q | (["BTC", "USD", "USDT", "USDC", "GBP"] | index($q)) != null)
    | [.symbol, .baseAsset, .quoteAsset, (.onboardDate // 0)]
    | @tsv
' "$EXCHANGE_INFO_JSON" > "$SYMBOLS_TSV"

if [ ! -s "$SYMBOLS_TSV" ]; then
    echo "No symbols matched required quote assets" >&2
    exit 1
fi

SYMBOL_LIMIT="${SYMBOL_LIMIT:-0}"
if ! [[ "$SYMBOL_LIMIT" =~ ^[0-9]+$ ]]; then
    echo "SYMBOL_LIMIT must be a non-negative integer" >&2
    exit 1
fi

if [ "$SYMBOL_LIMIT" -gt 0 ]; then
    LIMITED_SYMBOLS_TSV="$WORK_DIR/symbols-limited.tsv"
    head -n "$SYMBOL_LIMIT" "$SYMBOLS_TSV" > "$LIMITED_SYMBOLS_TSV"
    SYMBOLS_TSV="$LIMITED_SYMBOLS_TSV"
    echo "Limiting run to first $SYMBOL_LIMIT symbols"
fi

START_MS=""
END_EXCLUSIVE_MS=""
START_MS_END_MS="$(python3 - "$START_DATE" "$END_DATE" <<'PY'
import datetime as dt
import sys

start_date = dt.date.fromisoformat(sys.argv[1])
end_date = dt.date.fromisoformat(sys.argv[2])

start_dt = dt.datetime.combine(start_date, dt.time.min, tzinfo=dt.timezone.utc)
end_exclusive_dt = dt.datetime.combine(end_date + dt.timedelta(days=1), dt.time.min, tzinfo=dt.timezone.utc)

print(int(start_dt.timestamp() * 1000), int(end_exclusive_dt.timestamp() * 1000))
PY
)"
START_MS="${START_MS_END_MS%% *}"
END_EXCLUSIVE_MS="${START_MS_END_MS##* }"

echo "time,from_currency_ticker,from_currency_name,to_currency_ticker,to_currency_name,value" > "$RAW_PRICES_FILE"

is_filtered_base_asset() {
    local base_asset="$1"

    if [[ "$base_asset" == *DOWN ]] || [[ "$base_asset" == *UP ]] || [[ "$base_asset" == *BULL ]] || [[ "$base_asset" == *BEAR ]]; then
        return 0
    fi

    if [ "${#base_asset}" -ge 4 ] && [[ "$base_asset" == *B ]]; then
        return 0
    fi

    return 1
}

append_rows_from_klines_json() {
    local json_path="$1"
    local base_asset="$2"
    local quote_asset="$3"
    local chunk_start_ms="$4"
    local end_exclusive_ms="$5"
    local output_csv="$6"

    python3 - "$json_path" "$base_asset" "$quote_asset" "$chunk_start_ms" "$end_exclusive_ms" "$output_csv" <<'PY'
import csv
import datetime as dt
from decimal import Decimal, ROUND_HALF_UP, getcontext
import json
import sys

getcontext().prec = 60

json_path, base_asset, quote_asset, chunk_start_ms_s, end_exclusive_ms_s, output_csv = sys.argv[1:7]
chunk_start_ms = int(chunk_start_ms_s)
end_exclusive_ms = int(end_exclusive_ms_s)

very_small = Decimal("0.00000000000000000001")
scale = Decimal("0.00000000000000000001")

with open(json_path, encoding="utf-8") as infile:
    payload = json.load(infile)

if isinstance(payload, dict):
    code = payload.get("code")
    msg = payload.get("msg", "unknown API error")
    raise SystemExit(f"Binance API error {code}: {msg}")

if not isinstance(payload, list):
    raise SystemExit("Unexpected Binance klines payload")

rows_written = 0
max_open_time = -1

with open(output_csv, "a", newline="", encoding="utf-8") as outfile:
    writer = csv.writer(outfile)

    for row in payload:
        if len(row) < 5:
            continue

        open_time_ms = int(row[0])
        if open_time_ms < chunk_start_ms or open_time_ms >= end_exclusive_ms:
            continue

        open_price = Decimal(str(row[1]))
        high_price = Decimal(str(row[2]))
        low_price = Decimal(str(row[3]))
        close_price = Decimal(str(row[4]))

        avg_price = (open_price + high_price + low_price + close_price) / Decimal(4)
        if avg_price <= 0:
            avg_price = very_small

        avg_price = avg_price.quantize(scale, rounding=ROUND_HALF_UP)
        day = dt.datetime.fromtimestamp(open_time_ms / 1000, tz=dt.timezone.utc).date()

        writer.writerow(
            [
                f"{day.isoformat()} 00:00:00",
                base_asset,
                base_asset,
                quote_asset,
                quote_asset,
                format(avg_price, "f"),
            ]
        )

        rows_written += 1
        if open_time_ms > max_open_time:
            max_open_time = open_time_ms

print(f"{rows_written}\t{max_open_time}")
PY
}

SYMBOL_COUNT="$(wc -l < "$SYMBOLS_TSV" | tr -d ' ')"
symbol_index=0
symbols_skipped_by_name_filter=0
symbols_skipped_not_listed=0
api_requests=0
api_download_failures=0
api_parse_failures=0
api_requests_succeeded=0

while IFS=$'\t' read -r symbol base_asset quote_asset onboard_ms; do
    symbol_index=$((symbol_index + 1))

    if is_filtered_base_asset "$base_asset"; then
        symbols_skipped_by_name_filter=$((symbols_skipped_by_name_filter + 1))
        continue
    fi

    normalized_quote="$quote_asset"
    if [ "$normalized_quote" = "USDT" ]; then
        normalized_quote="USD"
    fi

    adjusted_start_ms="$START_MS"
    if [ "$onboard_ms" -gt "$adjusted_start_ms" ]; then
        adjusted_start_ms="$onboard_ms"
    fi

    if [ "$adjusted_start_ms" -ge "$END_EXCLUSIVE_MS" ]; then
        symbols_skipped_not_listed=$((symbols_skipped_not_listed + 1))
        continue
    fi

    echo "Processing ${symbol} (${symbol_index}/${SYMBOL_COUNT})"

    current_start_ms="$adjusted_start_ms"
    page=0

    while [ "$current_start_ms" -lt "$END_EXCLUSIVE_MS" ]; do
        page=$((page + 1))
        api_json_path="$WORK_DIR/${symbol}-${page}.json"
        api_url="https://api.binance.com/api/v3/klines?symbol=${symbol}&interval=1d&startTime=${current_start_ms}&endTime=${END_EXCLUSIVE_MS}&limit=1000"

        api_requests=$((api_requests + 1))

        if ! download_to_file "$api_url" "$api_json_path"; then
            api_download_failures=$((api_download_failures + 1))
            break
        fi

        parse_result=""
        if ! parse_result="$(append_rows_from_klines_json "$api_json_path" "$base_asset" "$normalized_quote" "$current_start_ms" "$END_EXCLUSIVE_MS" "$RAW_PRICES_FILE")"; then
            api_parse_failures=$((api_parse_failures + 1))
            break
        fi

        api_requests_succeeded=$((api_requests_succeeded + 1))

        rows_written="${parse_result%%$'\t'*}"
        last_open_time="${parse_result##*$'\t'}"

        if [ "$rows_written" -eq 0 ]; then
            break
        fi

        next_start_ms=$((last_open_time + 86400000))
        if [ "$next_start_ms" -le "$current_start_ms" ]; then
            echo "Pagination safety stop hit for $symbol" >&2
            api_parse_failures=$((api_parse_failures + 1))
            break
        fi

        current_start_ms="$next_start_ms"

        if [ "$rows_written" -lt 1000 ]; then
            break
        fi

        sleep "$REQUEST_DELAY_SECONDS"
    done
done < "$SYMBOLS_TSV"

PRICES_CSV="$OUTPUT_DIR/crypto_currency_prices_${START_DATE}_to_${END_DATE}.csv"
CURRENCIES_CSV="$OUTPUT_DIR/crypto_currencies_${START_DATE}_to_${END_DATE}.csv"

python3 - "$RAW_PRICES_FILE" "$PRICES_CSV" "$CURRENCIES_CSV" <<'PY'
import csv
import datetime as dt
from collections import defaultdict
from decimal import Decimal, ROUND_HALF_UP, getcontext
import sys

getcontext().prec = 60

raw_path, prices_path, currencies_path = sys.argv[1], sys.argv[2], sys.argv[3]
scale = Decimal("0.00000000000000000001")

grouped = defaultdict(list)

with open(raw_path, newline="", encoding="utf-8") as infile:
    reader = csv.DictReader(infile)
    for row in reader:
        key = (
            row["time"],
            row["from_currency_ticker"],
            row["to_currency_ticker"],
        )
        grouped[key].append(Decimal(row["value"]))

rows = []
currencies = set()

for (time_value, from_ticker, to_ticker), prices in grouped.items():
    avg_price = (sum(prices) / Decimal(len(prices))).quantize(scale, rounding=ROUND_HALF_UP)

    rows.append(
        {
            "time": time_value,
            "from_currency_ticker": from_ticker,
            "from_currency_name": from_ticker,
            "to_currency_ticker": to_ticker,
            "to_currency_name": to_ticker,
            "value": format(avg_price, "f"),
        }
    )

    currencies.add(from_ticker)
    currencies.add(to_ticker)

rows.sort(key=lambda row: (row["time"], row["from_currency_ticker"], row["to_currency_ticker"]))

with open(prices_path, "w", newline="", encoding="utf-8") as outfile:
    fieldnames = [
        "time",
        "from_currency_ticker",
        "from_currency_name",
        "to_currency_ticker",
        "to_currency_name",
        "value",
    ]
    writer = csv.DictWriter(outfile, fieldnames=fieldnames)
    writer.writeheader()
    writer.writerows(rows)

updated_at = dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%d %H:%M:%S")

with open(currencies_path, "w", newline="", encoding="utf-8") as outfile:
    fieldnames = ["ticker", "name", "updated_at"]
    writer = csv.DictWriter(outfile, fieldnames=fieldnames)
    writer.writeheader()
    for ticker in sorted(currencies):
        writer.writerow({"ticker": ticker, "name": ticker, "updated_at": updated_at})
PY

price_rows=0
currency_rows=0

if [ -f "$PRICES_CSV" ]; then
    price_rows=$(( $(wc -l < "$PRICES_CSV") - 1 ))
fi

if [ -f "$CURRENCIES_CSV" ]; then
    currency_rows=$(( $(wc -l < "$CURRENCIES_CSV") - 1 ))
fi

echo
echo "Backfill complete"
echo "Symbols examined: $SYMBOL_COUNT"
echo "Symbols skipped (leveraged pattern): $symbols_skipped_by_name_filter"
echo "Symbols skipped (listed after end date): $symbols_skipped_not_listed"
echo "API requests attempted: $api_requests"
echo "API requests succeeded: $api_requests_succeeded"
echo "API download failures: $api_download_failures"
echo "API parse failures: $api_parse_failures"
echo "Price rows written: $price_rows"
echo "Currency rows written: $currency_rows"
echo "Price CSV: $PRICES_CSV"
echo "Currency CSV: $CURRENCIES_CSV"
