#!/usr/bin/env bash

(
    cd cmd/pricewarp && go build -o ../../bin/pricewarp
)
bin/pricewarp &
server_pid=$!

echo $server_pid

# shellcheck disable=SC2034
inotifywait -r -m -e close_write . |
while read -r directory events filename; do
    if [[ $directory == ./template* ]] \
    || [[ $directory == ./internal* ]] \
    || [[ $directory == ./cmd* ]]; then
        if [[ $filename == *.go ]] || [[ $filename == *.tmpl ]]; then
            kill $server_pid
            sleep 1

            (
                cd cmd/pricewarp && go build -o ../../bin/pricewarp
            )
            bin/pricewarp &
            server_pid=$!
        fi
    fi
done
