#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/debug"
# This test validates the debug command
os::cmd::expect_success 'oc create -f test/integration/testdata/test-deployment-config.yaml'
os::cmd::expect_success_and_text "oc debug dc/test-deployment-config -o yaml" '\- /bin/sh'
os::cmd::expect_success_and_text "oc debug dc/test-deployment-config --keep-annotations -o yaml" 'annotations:'
os::cmd::expect_success_and_text "oc debug dc/test-deployment-config --as-root -o yaml" 'runAsUser: 0'
os::cmd::expect_success_and_text "oc debug dc/test-deployment-config --as-root=false -o yaml" 'runAsNonRoot: true'
os::cmd::expect_success_and_text "oc debug dc/test-deployment-config --as-user=1 -o yaml" 'runAsUser: 1'
os::cmd::expect_success_and_text "oc debug dc/test-deployment-config --keep-liveness --keep-readiness -o yaml" ''
os::cmd::expect_success_and_text "oc debug dc/test-deployment-config -o yaml -- /bin/env" '\- /bin/env'
os::cmd::expect_success_and_text "oc debug -t dc/test-deployment-config -o yaml" 'stdinOnce'
os::cmd::expect_success_and_text "oc debug -t dc/test-deployment-config -o yaml" 'tty'
os::cmd::expect_success_and_text "oc debug --v=8 -t dc/test-deployment-config -o yaml" "Response Headers"
os::cmd::expect_success_and_not_text "oc debug --tty=false dc/test-deployment-config -o yaml" 'tty'
os::cmd::expect_success_and_not_text "oc debug dc/test-deployment-config -o yaml -- /bin/env" 'stdin'
os::cmd::expect_success_and_not_text "oc debug dc/test-deployment-config -o yaml -- /bin/env" 'tty'
os::cmd::expect_failure_and_text "oc debug dc/test-deployment-config --node-name=invalid -- /bin/env" 'on node "invalid"'
# Does not require a real resource on the server
os::cmd::expect_success_and_not_text "oc debug -T -f examples/hello-openshift/hello-pod.json -o yaml" 'tty'
os::cmd::expect_success_and_text "oc debug -f examples/hello-openshift/hello-pod.json --keep-liveness --keep-readiness -o yaml" ''
os::cmd::expect_success_and_text "oc debug -f examples/hello-openshift/hello-pod.json -o yaml -- /bin/env" '\- /bin/env'
os::cmd::expect_success_and_not_text "oc debug -f examples/hello-openshift/hello-pod.json -o yaml -- /bin/env" 'stdin'
os::cmd::expect_success_and_not_text "oc debug -f examples/hello-openshift/hello-pod.json -o yaml -- /bin/env" 'tty'
# TODO: write a test that emulates a TTY to verify the correct defaulting of what the pod is created

# Ensure debug does not depend on a container actually existing for the selected resource.
# The command should not hang waiting for an attachable pod. Timeout each cmd after 10s.
os::cmd::expect_success 'oc create -f test/integration/testdata/test-replication-controller.yaml'
os::cmd::expect_success 'oc scale --replicas=0 rc/test-replication-controller'
os::cmd::expect_success_and_text "oc debug --request-timeout=10s -c ruby-helloworld --one-container rc/test-replication-controller -o jsonpath='{.metadata.name}'" 'test-replication-controller-debug'

os::cmd::expect_success 'oc scale --replicas=0 dc/test-deployment-config'
os::cmd::expect_success_and_text "oc debug --request-timeout=10s -c ruby-helloworld --one-container dc/test-deployment-config -o jsonpath='{.metadata.name}'" 'test-deployment-config'

tmp_deploy="$(mktemp)"
os::cmd::expect_success 'oc create -f - >> $tmp_deploy << __EOF__
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: test-deployment
  labels:
    deployment: test-deployment
spec:
  replicas: 0
  selector:
    matchLabels:
      deployment: test-deployment
  template:
    metadata:
      labels:
        deployment: test-deployment
      name: test-deployment
    spec:
      containers:
      - name: ruby-helloworld
        image: openshift/origin-pod
        imagePullPolicy: IfNotPresent
        resources: {}
status: {}
__EOF__'
os::cmd::expect_success_and_text "oc debug --request-timeout=10s -c ruby-helloworld --one-container deploy/test-deployment -o jsonpath='{.metadata.name}'" 'test-deployment-debug'

# re-scale existing resources
os::cmd::expect_success 'oc scale --replicas=1 dc/test-deployment-config'

os::cmd::expect_success 'oc create -f examples/image-streams/image-streams-centos7.json'
os::cmd::try_until_success 'oc get imagestreamtags wildfly:latest'
os::cmd::expect_success_and_text "oc debug istag/wildfly:latest -o yaml" 'image: docker.io/openshift/wildfly-120-centos7'
sha="$( oc get istag/wildfly:latest --template '{{ .image.metadata.name }}' )"
os::cmd::expect_success_and_text "oc debug isimage/wildfly@${sha} -o yaml" 'image: docker.io/openshift/wildfly-120-centos7'

echo "debug: ok"
os::test::junit::declare_suite_end
