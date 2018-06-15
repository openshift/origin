#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates,pv,pvc --all
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/volumes"
# This test validates the 'volume' command

os::cmd::expect_success 'oc create -f test/integration/testdata/test-deployment-config.yaml'
os::cmd::expect_success 'oc create -f test/testdata/rollingupdate-daemonset.yaml'

os::cmd::expect_success_and_text 'oc volume dc/test-deployment-config --list' 'vol1'
os::cmd::expect_success 'oc volume dc/test-deployment-config --add --name=vol0 -m /opt5'
os::cmd::expect_success 'oc set volume dc/test-deployment-config --add --name=vol2 --type=emptydir -m /opt'
os::cmd::expect_failure_and_text "oc set volume dc/test-deployment-config --add --name=vol1 --type=secret --secret-name='\$ecret' -m /data" 'overwrite to replace'
os::cmd::expect_success "oc set volume dc/test-deployment-config --add --name=vol10 --secret-name='my-secret' -m /data-2"
os::cmd::expect_success "oc set volume dc/test-deployment-config --add --name=vol11 --configmap-name='my-configmap' -m /data-21"
os::cmd::expect_success_and_text 'oc get dc/test-deployment-config -o jsonpath={.spec.template.spec.containers[0].volumeMounts}' '/data-21'
os::cmd::expect_success_and_text 'oc get dc/test-deployment-config -o jsonpath={.spec.template.spec.volumes[4].configMap}' 'my-configmap'
os::cmd::expect_success 'oc set volume dc/test-deployment-config --add --name=vol1 --type=emptyDir -m /data --overwrite'
os::cmd::expect_failure_and_text 'oc set volume dc/test-deployment-config --add -m /opt' "'/opt' already exists"
os::cmd::expect_success_and_text "oc set volume dc/test-deployment-config --add --name=vol2 -m /etc -c 'ruby' --overwrite" 'does not have any containers'
os::cmd::expect_success "oc set volume dc/test-deployment-config --add --name=vol2 -m /etc -c 'ruby*' --overwrite"
os::cmd::expect_success_and_text 'oc set volume dc/test-deployment-config --list --name=vol2' 'mounted at /etc'
os::cmd::expect_success_and_text 'oc set volume dc/test-deployment-config --add --name=vol3 -o yaml' 'name: vol3'
os::cmd::expect_failure_and_text 'oc set volume dc/test-deployment-config --list --name=vol3' 'volume "vol3" not found'
os::cmd::expect_failure_and_text 'oc set volume dc/test-deployment-config --remove' 'confirm for removing more than one volume'
os::cmd::expect_success 'oc set volume dc/test-deployment-config --remove --name=vol2'
os::cmd::expect_success_and_not_text 'oc set volume dc/test-deployment-config --list' 'vol2'
os::cmd::expect_success 'oc set volume dc/test-deployment-config --remove --confirm'
os::cmd::expect_success_and_not_text 'oc set volume dc/test-deployment-config --list' 'vol1'

# ensure that resources not present in all versions of a target group
# are still able to be encoded and patched accordingly
os::cmd::expect_success 'oc set volume ds/bind --add --name=vol2 --type=emptydir -m /opt'
os::cmd::expect_success 'oc set volume ds/bind --remove --name=vol2'

os::cmd::expect_success "oc volume dc/test-deployment-config --add -t 'secret' --secret-name='asdf' --default-mode '765'"
os::cmd::expect_success_and_text 'oc get dc/test-deployment-config -o jsonpath={.spec.template.spec.volumes[0]}' '501'
os::cmd::expect_success 'oc set volume dc/test-deployment-config --remove --confirm'

os::cmd::expect_failure "oc volume dc/test-deployment-config --add -t 'secret' --secret-name='asdf' --default-mode '888'"

os::cmd::expect_success "oc volume dc/test-deployment-config --add -t 'configmap' --configmap-name='asdf' --default-mode '123'"
os::cmd::expect_success_and_text 'oc get dc/test-deployment-config -o jsonpath={.spec.template.spec.volumes[0]}' '83'
os::cmd::expect_success 'oc set volume dc/test-deployment-config --remove --confirm'

os::cmd::expect_success_and_text 'oc get pvc --no-headers | wc -l' '0'
os::cmd::expect_success 'oc volume dc/test-deployment-config --add --mount-path=/other --claim-size=1G'
os::cmd::expect_success 'oc set volume dc/test-deployment-config --add --mount-path=/second --type=pvc --claim-size=1G --claim-mode=rwo'
os::cmd::expect_success_and_text 'oc get pvc --no-headers | wc -l' '2'
# attempt to add the same volume mounted in /other, but with a subpath
# we are not using --overwrite, so expect a failure
os::cmd::expect_failure_and_text 'oc set volume dc/test-deployment-config --add --mount-path=/second --sub-path=foo' "'/second' already exists"
# add --sub-path and expect success and --sub-path added when using --overwrite
os::cmd::expect_success_and_text 'oc set volume dc/test-deployment-config --add --mount-path=/second --sub-path=foo --overwrite' 'deploymentconfig "test-deployment-config" updated'
os::cmd::expect_success_and_text "oc get dc/test-deployment-config -o jsonpath='{.spec.template.spec.containers[0].volumeMounts[*].subPath}'" 'foo'

# ensure that we can describe volumes of type ConfigMap
os::cmd::expect_success " echo 'apiVersion: v1
kind: DeploymentConfig
metadata:
  name: simple-dc
  creationTimestamp: null
  labels:
    name: test-deployment
spec:
  replicas: 1
  selector:
    name: test-deployment
  template:
    metadata:
      labels:
        name: test-deployment
    spec:
      containers:
      - image: openshift/origin-ruby-sample
        name: helloworld
' | oc create -f -"

os::cmd::expect_success_and_text 'oc get dc simple-dc' 'simple-dc'
os::cmd::expect_success 'oc create cm cmvol'
os::cmd::expect_success 'oc set volume dc/simple-dc --add --name=cmvolume --type=configmap --configmap-name=cmvol'
os::cmd::expect_success_and_text 'oc volume dc/simple-dc' 'configMap/cmvol as cmvolume'

# command alias
os::cmd::expect_success 'oc volumes --help'
os::cmd::expect_success 'oc set volumes --help'
os::cmd::expect_success 'oc set volumes dc/test-deployment-config --list'

os::cmd::expect_success 'oc delete dc/test-deployment-config'
echo "volumes: ok"
os::test::junit::declare_suite_end
