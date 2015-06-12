#!/bin/bash

# Configuration script for data population

# The server to login to when provisioning users
export OPENSHIFT_SERVER=https://10.0.2.15:8443

# The admin user to populate
export OPENSHIFT_ADMIN_CONFIG=/openshift.local.config/master/admin.kubeconfig

# The ca cert to present when provisioning users
export OPENSHIFT_CA_CERT=/openshift.local.config/master/ca.crt

# The number of users that are in the system
export NUM_USERS=10

# The user name prefix
export USER_NAME_PREFIX=hal-

# The number of projects that are in the system
export NUM_PROJECTS=5

# The project name prefix
export PROJECT_NAME_PREFIX=project-