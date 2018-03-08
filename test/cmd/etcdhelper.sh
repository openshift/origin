#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

full_url="${API_SCHEME}://${API_HOST}:${ETCD_PORT}"
etcd_client_cert="${MASTER_CONFIG_DIR}/master.etcd-client.crt"
etcd_client_key="${MASTER_CONFIG_DIR}/master.etcd-client.key"
ca_bundle="${MASTER_CONFIG_DIR}/ca-bundle.crt"

os::test::junit::declare_suite_start "etcdhelper"

os::util::ensure::built_binary_exists 'etcdhelper' >&2

# create resources
os::cmd::expect_success 'oc new-project etcdhelper'
os::cmd::expect_success 'oc create imagestream python'

# verify its existence
os::cmd::expect_success_and_text 'etcdhelper --cert "${etcd_client_cert}" --key "${etcd_client_key}" --cacert "${ca_bundle}" --endpoint "${full_url}" ls /openshift.io' '/openshift.io/imagestreams/etcdhelper/python'
os::cmd::expect_success_and_text 'etcdhelper --cert "${etcd_client_cert}" --key "${etcd_client_key}" --cacert "${ca_bundle}" --endpoint "${full_url}" get /openshift.io/imagestreams/etcdhelper/python' 'Kind=ImageStream'

# cleanup
os::cmd::expect_success 'oc delete project etcdhelper'

os::test::junit::declare_suite_end
