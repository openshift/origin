#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/observe"

# basic scenarios
os::cmd::expect_failure_and_text 'oc observe' 'you must specify at least one argument containing the resource to observe'
os::cmd::expect_success_and_text 'oc observe serviceaccounts --once' 'Sync ended'
os::cmd::expect_success_and_text 'oc observe daemonsets --once' 'Nothing to sync, exiting immediately'
os::cmd::expect_success_and_text 'oc observe services --once --all-namespaces' 'default kubernetes'
os::cmd::expect_success_and_text 'oc observe services --once --all-namespaces --print-metrics-on-exit' 'observe_counts{type="Sync"}'
os::cmd::expect_failure_and_text 'oc observe services --once --names echo' '\-\-delete and \-\-names must both be specified'
os::cmd::expect_success_and_text 'oc observe services --exit-after=1s' 'Shutting down after 1s ...'
os::cmd::expect_success_and_text 'oc observe services --exit-after=1s --all-namespaces --print-metrics-on-exit' 'observe_counts{type="Sync"}'
os::cmd::expect_success_and_text 'oc observe services --once --all-namespaces' 'default kubernetes'
# TODO: fix #31755 and make this a --once test
os::cmd::expect_success_and_text 'oc observe services --exit-after=3s --all-namespaces --names echo --names default/notfound --delete echo --delete remove' 'remove default notfound'

# error counting
os::cmd::expect_failure_and_text 'oc observe services --exit-after=1m --all-namespaces --maximum-errors=1 -- /bin/sh -c "exit 1"' 'reached maximum error limit of 1, exiting'
os::cmd::expect_failure_and_text 'oc observe services --exit-after=1m --all-namespaces --retry-on-exit-code=2 --maximum-errors=1 --loglevel=4 -- /bin/sh -c "exit 2"' 'retrying command: exit status 2'

# argument templates
os::cmd::expect_success_and_text 'oc observe services --once --all-namespaces -a "{ .spec.clusterIP }"' '172.30.0.1'
os::cmd::expect_success_and_text 'oc observe services --once --all-namespaces -a "{{ .spec.clusterIP }}" --output=gotemplate' '172.30.0.1'
os::cmd::expect_success_and_text 'oc observe services --once --all-namespaces -a "bad{ .metadata.annotations.unset }key"' 'badkey'
os::cmd::expect_failure_and_text 'oc observe services --once --all-namespaces -a "bad{ .metadata.annotations.unset }key" --strict-templates' 'annotations is not found'
os::cmd::expect_success_and_text 'oc observe services --once --all-namespaces -a "{{ .unknown }}" --output=gotemplate' '""'
os::cmd::expect_success_and_text 'oc observe services --once --all-namespaces -a "bad{{ or (.unknown) \"\" }}key" --output=gotemplate' 'badkey'
os::cmd::expect_success_and_text 'oc observe services --once --all-namespaces -a "bad{{ .unknown }}key" --output=gotemplate --strict-templates' '\<no value\>'

echo "observe: ok"
os::test::junit::declare_suite_end