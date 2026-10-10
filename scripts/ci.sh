#!/bin/sh

set -e

echo "### CHECK FORMATTING ###"
gofmt_out=$(gofmt -l .)
if [ -n "$gofmt_out" ]; then
  echo "Unformatted files were found. Fix them with 'gofmt -w .':" >&2
  echo "$gofmt_out" >&2
  exit 1
fi

echo "### CHECK SWAGGER DOCS ###"
go tool swag init -g cmd/server/main.go -o docs --parseInternal --parseDependency -q
if ! git diff --exit-code -- docs/; then
  echo "Swagger docs out of date. Check the README for the 'swag init' command to run" >&2
  exit 1
fi

echo "### BUILD, VET, AND TEST ###"
go build ./...
go vet ./...
go test ./...

printf '\033[1;32mall checks passed\033[0m\n'
