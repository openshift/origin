#!/bin/bash

# Limits

# Populates each project with a limit

source $(dirname "${BASH_SOURCE}")/common.sh

echo "Populating limits"

OPENSHIFTCONFIG=${OPENSHIFT_ADMIN_CONFIG}
LIMIT=$(dirname "${BASH_SOURCE}")/limit.yaml

for ((i=1; i <=$NUM_PROJECTS; i++))
do
  openshift cli create -f $LIMIT --namespace=${PROJECT_NAME_PREFIX}${i}
done

echo "Done"