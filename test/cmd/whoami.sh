#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/whoami"
# This test validates the whoami command's --show-server flag
os::cmd::expect_success_and_text 'oc whoami --show-server' 'http(s)?:\/\/[0-9\.]+\:[0-9]+'

echo "whoami: ok"
os::test::junit::declare_suite_end
