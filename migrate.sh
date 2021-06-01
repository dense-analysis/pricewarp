#!/usr/bin/env bash

set -eu

source env.sh

exec go run cmd/migrate/migrate.go "$@"
