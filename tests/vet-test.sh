#!/bin/sh
set -e
if output=$(go vet ./... 2>&1 && go test ./... 2>&1); then
    echo "ok"
else
    echo "fail"
    echo "$output"
    false
fi
