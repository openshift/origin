#!/bin/bash

# Configuration script for data population

# The server to login to when provisioning users
export OPENSHIFT_SERVER="${OPENSHIFT_SERVER:-https://10.0.2.15:8443}"

# The admin user to populate
export OPENSHIFT_ADMIN_CONFIG="${OPENSHIFT_ADMIN_CONFIG:-/origin.local.config/master/admin.kubeconfig}"

# The ca cert to present when provisioning users
export OPENSHIFT_CA_CERT="${OPENSHIFT_CA_CERT:-/origin.local.config/master/ca.crt}"

# The number of users that are in the system
export NUM_USERS="${NUM_USERS:-10}"

# The user name prefix
export USER_NAME_PREFIX=hal-

# The number of projects that are in the system
export NUM_PROJECTS="${NUM_PROJECTS:-3}"

# The project name prefix
export PROJECT_NAME_PREFIX=project-

# How many concurrent CLI requests to make
export MAX_PROCS=4
