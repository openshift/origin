#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all --all
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/rsync"
# This test validates the rsync command
os::cmd::expect_success 'oc create -f - << __EOF__
apiVersion: v1
kind: Pod
metadata:
  name: valid-pod
  labels:
    name: valid-pod
spec:
  containers:
  - name: kubernetes-serve-hostname
    image: k8s.gcr.io/serve_hostname
    resources:
      limits:
        cpu: "1"
        memory: 512Mi
__EOF__'

temp_dir=$(mktemp -d)
include_file=$(mktemp -p $temp_dir)
exclude_file=$(mktemp -p $temp_dir)

# we don't actually have a kubelet running, so no "tar" binary will be available in container because there will be no container.
# instead, ensure that the tar command to be executed is formatted correctly based on our --include and --exclude values
os::cmd::expect_failure_and_text "oc rsync --strategy=tar --include=$include_file --exclude=$exclude_file $temp_dir valid-pod:/tmp --loglevel 4" "running command: tar.*\*\*\/$include_file.*--exclude=$exclude_file"

echo "rsync: ok"
os::test::junit::declare_suite_end
