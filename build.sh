#!/usr/bin/env bash

set -eu

for executable in migrate ingest notify adduser pricewarp; do
    (
        echo "Building bin/$executable..."
        cd "cmd/$executable"
        go build -ldflags "-s -w" -o "../../bin/$executable"
    )
done
