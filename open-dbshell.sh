#!/bin/bash

# shellcheck disable=SC2046
export $(xargs < .env)

PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" "$DB_NAME" "$DB_USERNAME"
