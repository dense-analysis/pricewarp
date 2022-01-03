#!/usr/bin/env bash

set -eu

(
    echo "Building bin/migrate..."
    cd cmd/migrate
    go build -o ../../bin/migrate
)
