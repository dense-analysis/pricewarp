#!/usr/bin/env bash

set -eu

for executable in ingest notify adduser pricewarp; do
    (
        echo "Building bin/$executable..."
        cd "cmd/$executable"
        go build -ldflags "-s -w" -o "../../bin/$executable"
    )
done
