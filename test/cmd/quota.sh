#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/quota"

# Cleanup cluster resources created by this test suite
(
  set +e
  oc delete project foo bar asmail
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/quota/clusterquota"

os::cmd::expect_success 'oc new-project foo --as=deads'
os::cmd::expect_success 'oc label namespace/foo owner=deads'
os::cmd::expect_success 'oc create clusterquota for-deads --project-label-selector=owner=deads --hard=secrets=10'
os::cmd::try_until_text 'oc get appliedclusterresourcequota -n foo --as deads -o name' "for-deads"
os::cmd::try_until_text 'oc describe appliedclusterresourcequota/for-deads -n foo --as deads' "secrets.*9"

os::cmd::expect_failure_and_text 'oc create clusterquota for-deads-malformed --project-annotation-selector="openshift.#$%/requester=deads"' "prefix part must match the regex"
os::cmd::expect_failure_and_text 'oc create clusterquota for-deads-malformed --project-annotation-selector=openshift.io/requester=deads,openshift.io/novalue' "Malformed annotation selector"
os::cmd::expect_success 'oc create clusterquota for-deads-by-annotation --project-annotation-selector=openshift.io/requester=deads --hard=secrets=50'
os::cmd::expect_success 'oc create clusterquota for-deads-email-by-annotation --project-annotation-selector=openshift.io/requester=deads@deads.io --hard=secrets=50'
os::cmd::expect_success 'oc create clusterresourcequota annotation-value-with-commas --project-annotation-selector="openshift.io/requester=deads,\"openshift.io/withcomma=yes,true,1\"" --hard=pods=10'
os::cmd::expect_success 'oc new-project bar --as=deads'
os::cmd::expect_success 'oc new-project asmail --as=deads@deads.io'
os::cmd::try_until_text 'oc get appliedclusterresourcequota -n bar --as deads -o name' "for-deads-by-annotation"
os::cmd::try_until_text 'oc get appliedclusterresourcequota -n foo --as deads -o name' "for-deads-by-annotation"
os::cmd::try_until_text 'oc get appliedclusterresourcequota -n asmail --as deads@deads.io -o name' "for-deads-email-by-annotation"
os::cmd::try_until_text 'oc describe appliedclusterresourcequota/for-deads-by-annotation -n bar --as deads' "secrets.*[1-4][0-9]"

os::cmd::expect_success 'oc delete project foo'
os::cmd::expect_success 'oc delete project bar'
os::cmd::expect_success 'oc delete project asmail'

echo "clusterquota: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/quota/imagestreams"

os::cmd::expect_success 'oc new-project foo-2 --as=deads'
os::cmd::expect_success 'oc create quota -n foo-2 is-quota --hard openshift.io/imagestreams=1'
os::cmd::try_until_success 'oc tag -n foo-2 openshift/hello-openshift myis2:v2'
os::cmd::expect_failure_and_text 'oc tag -n foo-2 busybox mybox:v1' "exceeded quota"

echo "imagestreams: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_end
