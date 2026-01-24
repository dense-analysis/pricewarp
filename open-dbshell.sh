#!/bin/bash

# shellcheck disable=SC2046
export $(xargs < .env)

clickhouse-client --host "$DB_HOST" --port "$DB_PORT" --user "$DB_USERNAME" --password "$DB_PASSWORD" --database "$DB_NAME"
