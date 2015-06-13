#!/bin/bash

# Quotas

# Populates each project with a quota

source $(dirname "${BASH_SOURCE}")/common.sh

echo "Populating quotas"

export KUBECONFIG=${OPENSHIFT_ADMIN_CONFIG}

QUOTA=$(dirname "${BASH_SOURCE}")/quota.yaml

for ((i=1; i <=$NUM_PROJECTS; i++))
do
  openshift cli create -f $QUOTA --namespace=${PROJECT_NAME_PREFIX}${i}
done

echo "Done"