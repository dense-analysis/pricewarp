#!/usr/bin/env bash

set -eu

#shellcheck disable=SC2046
export $(xargs < .env)

# Use the HOME directory ClickHouse client if we can't find it.
if ! command -v clickhouse-client &> /dev/null; then
    clickhouse-client() {
        ~/clickhouse/clickhouse client "$@"
    }
fi

clickhouse-client --multiquery \
    --host "$DB_HOST" \
    --port "$DB_PORT" \
    --user "$DB_USERNAME" \
    --password "$DB_PASSWORD" \
    --database "$DB_NAME" \
    < sql/condense-prices.sql
