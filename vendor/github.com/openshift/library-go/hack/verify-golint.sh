#!/bin/bash

if ! command -v golint > /dev/null; then
    echo 'Can not find golint, install with:'
    echo 'go get -u github.com/golang/lint/golint'
    exit 1
fi

export IFS=$'\n'
all_packages=(
	"$(go list -e ./...)"
)
unset IFS

for p in "${all_packages[@]}"; do
    golint "$p"/*.go 2>/dev/null
done
