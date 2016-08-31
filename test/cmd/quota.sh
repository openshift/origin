#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/quota"

os::test::junit::declare_suite_start "cmd/quota/clusterquota"

os::cmd::expect_success 'oc new-project foo --as=deads'
os::cmd::expect_success 'oc label namespace/foo owner=deads'
os::cmd::expect_success 'oc create clusterquota for-deads --project-label-selector=owner=deads --hard=secrets=10'
os::cmd::try_until_text 'oc get appliedclusterresourcequota -n foo --as deads -o name' "for-deads"
os::cmd::try_until_text 'oc describe appliedclusterresourcequota/for-deads -n foo --as deads' "secrets.*9"


os::cmd::expect_success 'oc create clusterquota for-deads-by-annotation --project-annotation-selector=openshift.io/requester=deads --hard=secrets=50'
os::cmd::expect_success 'oc new-project bar --as=deads'
os::cmd::try_until_text 'oc get appliedclusterresourcequota -n bar --as deads -o name' "for-deads-by-annotation"
os::cmd::try_until_text 'oc get appliedclusterresourcequota -n foo --as deads -o name' "for-deads-by-annotation"
os::cmd::try_until_text 'oc describe appliedclusterresourcequota/for-deads-by-annotation -n bar --as deads' "secrets.*1[0-9]"

os::cmd::expect_success 'oc delete project foo'
os::cmd::expect_success 'oc delete project bar'

echo "clusterquota: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/quota/imagestreams"

os::cmd::expect_success 'oc new-project foo-2 --as=deads'
os::cmd::expect_success 'oc create quota -n foo-2 is-quota --hard openshift.io/imagestreams=1'
os::cmd::try_until_success 'oc tag -n foo-2 openshift/hello-openshift myis2:v2'
os::cmd::expect_failure_and_text 'oc tag -n foo-2 busybox mybox:v1' "Exceeded quota"

echo "imagestreams: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_end
