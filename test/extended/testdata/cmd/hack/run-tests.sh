#!/usr/bin/env bash

mkdir -p /tmp/tests/hack
cp -Lr /var/tests/hack/test-cmd.sh /var/tests/hack/lib /tmp/tests/hack
cp -Lr "$TESTS_DIR" "/tmp/tests/test"
TESTS_DIR="/tmp/tests/test/cmd" /tmp/tests/hack/test-cmd.sh
