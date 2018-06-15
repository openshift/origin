#!/bin/bash

# Projects

# Populates the system with projects

source $(dirname "${BASH_SOURCE}")/common.sh

echo "Populating projects"

export KUBECONFIG=${OPENSHIFT_ADMIN_CONFIG}

for ((i=1; i <=$NUM_PROJECTS; i++))
do
  number=$RANDOM
  let "number %= $NUM_USERS"
  ADMIN_USER=${USER_NAME_PREFIX}${number}
  oc adm new-project ${PROJECT_NAME_PREFIX}${i} --admin=$ADMIN_USER>/dev/null
done

echo "Done"