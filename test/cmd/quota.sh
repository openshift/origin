#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/quota"

# Cleanup cluster resources created by this test suite
(
  set +e
  oc delete project quota-{foo,bar,asmail,images}
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/quota/clusterquota"

os::cmd::expect_success 'oc new-project quota-foo --as=deads --as-group=system:authenticated --as-group=system:authenticated:oauth'
os::cmd::expect_success 'oc label namespace/quota-foo owner=deads'
os::cmd::expect_success 'oc create clusterquota for-deads --project-label-selector=owner=deads --hard=secrets=10'
os::cmd::try_until_text 'oc get appliedclusterresourcequota -n quota-foo --as deads -o name' "for-deads"
os::cmd::try_until_text 'oc get secrets -o name --all-namespaces; oc describe appliedclusterresourcequota/for-deads -n quota-foo --as deads' "secrets.*9"

os::cmd::expect_failure_and_text 'oc create clusterquota for-deads-malformed --project-annotation-selector="openshift.#$%/requester=deads"' "prefix part a DNS-1123 subdomain must consist of lower case alphanumeric characters"
os::cmd::expect_failure_and_text 'oc create clusterquota for-deads-malformed --project-annotation-selector=openshift.io/requester=deads,openshift.io/novalue' "Malformed annotation selector"
os::cmd::expect_success 'oc create clusterquota for-deads-by-annotation --project-annotation-selector=openshift.io/requester=deads --hard=secrets=50'
os::cmd::expect_success 'oc create clusterquota for-deads-email-by-annotation --project-annotation-selector=openshift.io/requester=deads@deads.io --hard=secrets=50'
os::cmd::expect_success 'oc create clusterresourcequota annotation-value-with-commas --project-annotation-selector="openshift.io/requester=deads,\"openshift.io/withcomma=yes,true,1\"" --hard=pods=10'
os::cmd::expect_success 'oc new-project quota-bar --as=deads  --as-group=system:authenticated --as-group=system:authenticated:oauth'
os::cmd::expect_success 'oc new-project quota-asmail --as=deads@deads.io  --as-group=system:authenticated --as-group=system:authenticated:oauth'
os::cmd::try_until_text 'oc get appliedclusterresourcequota -n quota-bar --as deads -o name' "for-deads-by-annotation"
os::cmd::try_until_text 'oc get appliedclusterresourcequota -n quota-foo --as deads -o name' "for-deads-by-annotation"
os::cmd::try_until_text 'oc get appliedclusterresourcequota -n quota-asmail --as deads@deads.io -o name' "for-deads-email-by-annotation"
# the point of the test is to make sure that clusterquota is counting correct and secrets are auto-created and countable
# the create_dockercfg controller can issue multiple creates if the token controller doesn't fill them in, but the creates are duplicates
# since an annotation tracks the intended secrets to be created.  That results in multi-counting quota until reconciliation runs
# do not go past 26.  If you get to 27, you might be selecting an extra namespace.
os::cmd::try_until_text 'oc get secrets -o name --all-namespaces; oc describe appliedclusterresourcequota/for-deads-by-annotation -n quota-bar --as deads' "secrets.*(1[0-9]|20|21|22|23|24|25|26)"
os::cmd::expect_success 'oc delete project quota-foo'
os::cmd::try_until_not_text 'oc get clusterresourcequota/for-deads-by-annotation -o jsonpath="{.status.namespaces[*].namespace}"' 'quota-foo'
os::cmd::expect_success 'oc delete project quota-bar'
os::cmd::expect_success 'oc delete project quota-asmail'

echo "clusterquota: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/quota/imagestreams"

os::cmd::expect_success 'oc new-project quota-images --as=deads  --as-group=system:authenticated --as-group=system:authenticated:oauth'
os::cmd::expect_success 'oc create quota -n quota-images is-quota --hard openshift.io/imagestreams=1'
os::cmd::try_until_success 'oc tag -n quota-images openshift/hello-openshift myis2:v2'
os::cmd::expect_failure_and_text 'oc tag -n quota-images busybox mybox:v1' "exceeded quota"
os::cmd::expect_success 'oc delete project quota-images'

echo "imagestreams: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_end
