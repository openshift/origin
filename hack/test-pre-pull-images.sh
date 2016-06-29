#!/bin/bash

echo "[INFO] Prepulling container images"

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
KUBE_TEST_E2E=${OS_ROOT}/Godeps/_workspace/src/k8s.io/kubernetes/test/e2e/

# https://github.com/kubernetes/kubernetes/blob/master/cluster/saltbase/salt/e2e-image-puller/e2e-image-puller.manifest
# CURRENT_LIST of images was created via (a modified version from the above reference):
# grep -Iiroh -e "gcr.io/google_.*" -e "openshift/origin:"  ${OS_ROOT}/test/ "${KUBE_TEST_E2E}" | awk '{print $1}' | sed -e "s/[,\")}]//g" | sed -e "s/:$//g" | sed -e "s/'$//g" | sort | uniq| tr '\n' ' '
# CURRENT_LIST is an auto generated (and really a guestimate) list and all image pulls might not work.

CURRENT_LIST="gcr.io/google_containers/busybox gcr.io/google_containers/busybox:1.24 gcr.io/google_containers/dnsutils:e2e gcr.io/google_containers/eptest:0.1 gcr.io/google_containers/fakegitserver:0.1 gcr.io/google_containers/hostexec:1.2 gcr.io/google_containers/iperf:e2e gcr.io/google_containers/jessie-dnsutils:e2e gcr.io/google_containers/liveness:e2e gcr.io/google_containers/mounttest:0.2 gcr.io/google_containers/mounttest:0.5 gcr.io/google_containers/mounttest:0.6 gcr.io/google_containers/mounttest-user:0.3 gcr.io/google_containers/netexec:1.4 gcr.io/google_containers/netexec:1.5 gcr.io/google_containers/nettest gcr.io/google_containers/nettest:1.7 gcr.io/google_containers/nginx:1.7.9 gcr.io/google_containers/nginx-slim:0.5 gcr.io/google_containers/n-way-http:1.0 gcr.io/google_containers/pause gcr.io/google_containers/pause:2.0 gcr.io/google_containers/pause-amd64:3.0 gcr.io/google_containers/porter:cd5cb5791ebaa8641955f0e8c2a9bed669b1eaab gcr.io/google_containers/portforwardtester:1.0 gcr.io/google_containers/redis:e2e gcr.io/google_containers/resource_consumer:beta2 gcr.io/google_containers/serve_hostname:v1.4 gcr.io/google_containers/servicelb:0.1 gcr.io/google_containers/test-webserver:e2e gcr.io/google_containers/ubuntu:14.04 gcr.io/google_containers/update-demo:kitten gcr.io/google_containers/update-demo:nautilus gcr.io/google_containers/volume-ceph:0.1 gcr.io/google_containers/volume-gluster:0.2 gcr.io/google_containers/volume-iscsi:0.1 gcr.io/google_containers/volume-nfs:0.6 gcr.io/google_containers/volume-rbd:0.1 gcr.io/google_samples/gb-redisslave:v1 openshift/origin "


NEW_LIST=$(grep -Iiroh -e "gcr.io/google_.*" -e "openshift/origin:"  ${OS_ROOT}/test/ "${KUBE_TEST_E2E}" | awk '{print $1}' | sed -e "s/[,\")}]//g" | sed -e "s/:$//g" | sed -e "s/'$//g" | sort | uniq| tr '\n' ' ')

if [ "$CURRENT_LIST" != "$NEW_LIST" ]; then
  echo "[WARNING] Prepulling image list should be updated"
fi

# List of images urls excluded from the working list
# gcr.io/google_containers/nettest

# CURRENT_WORKING_LIST is a working subset of CURRENT_LIST
CURRENT_WORKING_LIST="gcr.io/google_containers/busybox
gcr.io/google_containers/busybox:1.24
gcr.io/google_containers/dnsutils:e2e
gcr.io/google_containers/eptest:0.1
gcr.io/google_containers/fakegitserver:0.1
gcr.io/google_containers/hostexec:1.2
gcr.io/google_containers/iperf:e2e
gcr.io/google_containers/jessie-dnsutils:e2e
gcr.io/google_containers/liveness:e2e
gcr.io/google_containers/mounttest:0.2
gcr.io/google_containers/mounttest:0.5
gcr.io/google_containers/mounttest:0.6
gcr.io/google_containers/mounttest-user:0.3
gcr.io/google_containers/netexec:1.4
gcr.io/google_containers/netexec:1.5
gcr.io/google_containers/nettest:1.7
gcr.io/google_containers/nginx:1.7.9
gcr.io/google_containers/nginx-slim:0.5
gcr.io/google_containers/n-way-http:1.0
gcr.io/google_containers/pause
gcr.io/google_containers/pause:2.0
gcr.io/google_containers/pause-amd64:3.0
gcr.io/google_containers/porter:cd5cb5791ebaa8641955f0e8c2a9bed669b1eaab
gcr.io/google_containers/portforwardtester:1.0
gcr.io/google_containers/redis:e2e
gcr.io/google_containers/resource_consumer:beta2
gcr.io/google_containers/serve_hostname:v1.4
gcr.io/google_containers/servicelb:0.1
gcr.io/google_containers/test-webserver:e2e
gcr.io/google_containers/ubuntu:14.04
gcr.io/google_containers/update-demo:kitten
gcr.io/google_containers/update-demo:nautilus
gcr.io/google_containers/volume-ceph:0.1
gcr.io/google_containers/volume-gluster:0.2
gcr.io/google_containers/volume-iscsi:0.1
gcr.io/google_containers/volume-nfs:0.6
gcr.io/google_containers/volume-rbd:0.1
gcr.io/google_samples/gb-redisslave:v1
openshift/origin"

# Manual list for images not to be found by an automated search method above
# or for any other reasons
MANUAL_LIST=""

for i in $CURRENT_WORKING_LIST $MANUAL_LIST; do
	echo $(date '+%X') pulling $i
	docker pull $i 1>/dev/null
done

exit 0
