#!/bin/bash

# Replication Controllers

# Populates each project with a replication controller whose replica size is 0
# Intended to test large numbers of replication controllers in system

source $(dirname "${BASH_SOURCE}")/common.sh

echo "Populating replication controllers"

export KUBECONFIG=${OPENSHIFT_ADMIN_CONFIG}

RC=$(dirname "${BASH_SOURCE}")/replication-controller.yaml

for ((i=1; i <=$NUM_PROJECTS; i++))
do
  openshift cli create -f $RC --namespace=${PROJECT_NAME_PREFIX}${i}
done

echo "Done"