#!/bin/bash

CURRENT_DIR="$(pwd)"
export HEKETI_SERVER_BUILD_DIR=../../..
FUNCTIONAL_DIR="${CURRENT_DIR}/.."
export HEKETI_SERVER="${FUNCTIONAL_DIR}/heketi-server"

source "${FUNCTIONAL_DIR}/lib.sh"

teardown

