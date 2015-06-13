#!/bin/bash

# Services

# Populates each project with a service whose label selector by default is not resolving any endpoints
# Intended to test large numbers of services

source $(dirname "${BASH_SOURCE}")/common.sh

echo "Populating services"

export KUBECONFIG=${OPENSHIFT_ADMIN_CONFIG}

SERVICE=$(dirname "${BASH_SOURCE}")/service.yaml

for ((i=1; i <=$NUM_PROJECTS; i++))
do
  openshift cli create -f $SERVICE --namespace=${PROJECT_NAME_PREFIX}${i}
done

echo "Done"