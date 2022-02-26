#!/usr/bin/env bash

set -eu

#shellcheck disable=SC2046
export $(xargs < .env)

PGPASSWORD="$DB_PASSWORD" psql -q -h "$DB_HOST" "$DB_NAME" "$DB_USERNAME" \
    < sql/condense-prices.sql
