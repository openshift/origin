#!/bin/bash

GOPACKAGES="$(go list ./... | grep -v vendor)"
# shellcheck disable=SC2086
exec go vet ${GOPACKAGES}
