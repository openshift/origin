#!/bin/bash

# Users
# Populates the system with users

source $(dirname "${BASH_SOURCE}")/common.sh

echo "Populating users"

for ((i=1; i <=$NUM_USERS; i++))
do  
  USERNAME=${USER_NAME_PREFIX}${i}
  USERCONFIG=/tmp/${USERNAME}.config
  openshift cli config view --minify --raw -o yaml > ${USERCONFIG}
  KUBECONFIG=${USERCONFIG} oc login --username=$USERNAME --password=whocares
done

echo "Done"