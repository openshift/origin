#!/bin/bash

# Projects

# Populates the system with projects

source $(dirname "${BASH_SOURCE}")/common.sh

echo "Populating projects"

OPENSHIFTCONFIG=${OPENSHIFT_ADMIN_CONFIG}

for ((i=1; i <=$NUM_PROJECTS; i++))
do
  ADMIN_USER=${USER_NAME_PREFIX}$(shuf -i 1-$NUM_USERS -n 1)
  openshift admin new-project ${PROJECT_NAME_PREFIX}${i} --admin=$ADMIN_USER>/dev/null
done

echo "Done"