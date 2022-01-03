#!/usr/bin/env bash

set -eu

for executable in migrate ingest notify; do
    (
        echo "Building bin/$executable..."
        cd "cmd/$executable"
        go build -o "../../bin/$executable"
    )
done
