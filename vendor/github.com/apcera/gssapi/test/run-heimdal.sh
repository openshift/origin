#!/bin/bash -eu

# Copyright 2013-2015 Apcera Inc. All rights reserved.

REUSE_DOCKER_IMAGES="" \
SERVICE_LOG_FILTER="" \
EXT_KDC_HOST="" \
EXT_KDC_PORT="" \
KEYTAB_FILE="" \
SERVICE_NAME="HTTP/auth.www.xample.test" \
REALM_NAME="XAMPLE.TEST" \
DOMAIN_NAME="xample.test" \
USER_NAME="testuser" \
USER_PASSWORD="P@ssword!" \
CLIENT_IN_CONTAINER="yes" \
        ./run.sh


