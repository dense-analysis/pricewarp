#!/usr/bin/env bash

set -eu

# Build deployable binaries only.
# Intentionally excluded: cmd/chquery (run with `go run ./cmd/chquery`).
for executable in ingest notify adduser pricewarp; do
    (
        echo "Building bin/$executable..."
        cd "cmd/$executable"
        go build -ldflags "-s -w" -o "../../bin/$executable"
    )
done
