#!/bin/sh

set -e
vagrant up --no-provision "$@"
vagrant provision
