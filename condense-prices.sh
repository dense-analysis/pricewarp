#!/usr/bin/env bash

set -eu

#shellcheck disable=SC2046
export $(xargs < .env)

clickhouse-client --host "$DB_HOST" --port "$DB_PORT" --user "$DB_USERNAME" --password "$DB_PASSWORD" --database "$DB_NAME" \
    < sql/condense-prices.sql
