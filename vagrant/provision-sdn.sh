#!/bin/bash
set -ex
source $(dirname $0)/provision-config.sh

# Setup openshift-sdn
echo "Setup openshift-sdn"
OSDN_BASE_PATH="Godeps/_workspace/src/github.com/openshift/openshift-sdn"
OSDN_CONTROLLER="${OSDN_BASE_PATH}/ovssubnet/controller"
KUBE_OSDN_PATH="/usr/libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet/"

pushd /vagrant/${OSDN_CONTROLLER}
  cp -f kube/bin/openshift-ovs-subnet /usr/bin/
  cp -f kube/bin/openshift-sdn-kube-subnet-setup.sh /usr/bin/

  mkdir -p ${KUBE_OSDN_PATH}
  cp -f kube/bin/openshift-ovs-subnet ${KUBE_OSDN_PATH}
  cp -f multitenant/bin/openshift-ovs-multitenant /usr/bin/
  cp -f multitenant/bin/openshift-sdn-multitenant-setup.sh /usr/bin/
popd

systemctl enable openvswitch
systemctl start openvswitch
