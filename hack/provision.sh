#!/bin/bash

# DESCRIPTION:
# This is a script that bootstraps basic content in the OpenShift server
# to facilitate development flows and users that want to kick the tires.
# 
# PROVISIONS:
# * Users
# ** test-cluster-admin
# ** test-admin
# * Docker registry
# * OpenShift router
# * OpenShift templates

# INSTRUCTIONS:
# $ git clone ...
# $ make all
# $ sudo <build output location>/openshift start
# $ hack/provision.sh

set -o errexit
set -o nounset
set -o pipefail

# Export necessary environment variables for resources created on openshift start
export ADMIN_KUBECONFIG=`pwd`/openshift.local.config/master/admin.kubeconfig
export REGISTRY_KUBECONFIG=`pwd`/openshift.local.config/master/openshift-registry.kubeconfig
export ROUTER_KUBECONFIG=`pwd`/openshift.local.config/master/openshift-router.kubeconfig
export CA=`pwd`/openshift.local.config/master

# Modify permissions on each config
sudo chmod -R a+rwx "$CA"
sudo chmod a+rwX "$ADMIN_KUBECONFIG"
sudo chmod a+rwX "$REGISTRY_KUBECONFIG"
sudo chmod a+rwX "$ROUTER_KUBECONFIG"

# Configure test users
oadm policy add-cluster-role-to-user --config="$ADMIN_KUBECONFIG" cluster-admin test-cluster-admin
oadm policy add-role-to-user --config="$ADMIN_KUBECONFIG" -n default view test-admin

# Install a docker registry on the cluster
oadm registry --config="$ADMIN_KUBECONFIG" --credentials="$REGISTRY_KUBECONFIG" --latest-images

# Install an OpenShift router
echo '{"kind":"ServiceAccount","apiVersion":"v1","metadata":{"name":"router"}}' | oc create --config="$ADMIN_KUBECONFIG" -n default -f -
oc get --config="$ADMIN_KUBECONFIG" scc privileged -o yaml > /tmp/priv.yaml
echo "- system:serviceaccount:default:router" >> /tmp/priv.yaml
oc replace --config="$ADMIN_KUBECONFIG" -f /tmp/priv.yaml
rm /tmp/priv.yaml
pushd /tmp
oadm ca create-server-cert --signer-cert=${CA}/ca.crt \
  --signer-key=${CA}/ca.key --signer-serial=${CA}/ca.serial.txt \
  --hostnames='*.router.default.svc.cluster.local' \
  --cert=router.crt --key=router.key \
  --config="$ADMIN_KUBECONFIG"
cat router.crt router.key ${CA}/ca.crt > router.pem
oadm router --config="$ADMIN_KUBECONFIG" --default-cert=router.pem --credentials="${ROUTER_KUBECONFIG}" --service-account=router
rm router.pem
popd

# Install image streams from origin
oc create --config="$ADMIN_KUBECONFIG" -n openshift -f ./examples/image-streams/image-streams-centos7.json
# oc create --config="$ADMIN_KUBECONFIG" -n openshift -f ./examples/image-streams/image-streams-rhel7.json

# Install templates from origin
oc create --config="$ADMIN_KUBECONFIG" -n openshift -f ./examples/db-templates
oc create --config="$ADMIN_KUBECONFIG" -n openshift -f ./examples/jenkins/jenkins-ephemeral-template.json
oc create --config="$ADMIN_KUBECONFIG" -n openshift -f ./examples/jenkins/jenkins-persistent-template.json

# Install templates from other example repositories
oc create --config="$ADMIN_KUBECONFIG" -n openshift -f https://raw.githubusercontent.com/openshift/cakephp-ex/master/openshift/templates/cakephp-mysql.json
oc create --config="$ADMIN_KUBECONFIG" -n openshift -f https://raw.githubusercontent.com/openshift/cakephp-ex/master/openshift/templates/cakephp.json
oc create --config="$ADMIN_KUBECONFIG" -n openshift -f https://raw.githubusercontent.com/openshift/dancer-ex/master/openshift/templates/dancer-mysql.json
oc create --config="$ADMIN_KUBECONFIG" -n openshift -f https://raw.githubusercontent.com/openshift/dancer-ex/master/openshift/templates/dancer.json
oc create --config="$ADMIN_KUBECONFIG" -n openshift -f https://raw.githubusercontent.com/openshift/rails-ex/master/openshift/templates/rails-postgresql.json
oc create --config="$ADMIN_KUBECONFIG" -n openshift -f https://raw.githubusercontent.com/openshift/django-ex/master/openshift/templates/django-postgresql.json
oc create --config="$ADMIN_KUBECONFIG" -n openshift -f https://raw.githubusercontent.com/openshift/django-ex/master/openshift/templates/django.json
oc create --config="$ADMIN_KUBECONFIG" -n openshift -f https://raw.githubusercontent.com/openshift/nodejs-ex/master/openshift/templates/nodejs-mongodb.json
oc create --config="$ADMIN_KUBECONFIG" -n openshift -f https://raw.githubusercontent.com/openshift/nodejs-ex/master/openshift/templates/nodejs.json

# Create a test project
oadm new-project --display-name="OpenShift 3 Sample" --description="This is an example project to demonstrate OpenShift v3" --admin=test-admin --config="$ADMIN_KUBECONFIG" test