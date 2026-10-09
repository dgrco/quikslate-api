#!/bin/sh

set -e

gofmt_out=$(gofmt -l .)
if [ -n "$gofmt_out" ]; then
  echo "Unformatted files were found. Fix them with 'gofmt -w .':" >&2
  echo "$gofmt_out" >&2
  exit 1
fi

go build ./...
go vet ./...
go test ./...
