#!/bin/bash -eu

# Copyright 2013-2015 Apcera Inc. All rights reserved.

# This script is used in the context of a docker VM when runnning the linux
# client test, and in the context of OS X when running on the Macintosh.  The
# following variables must be set (either via --link or explicitely)
#       KDC_PORT_88_TCP_ADDR
#       KDC_PORT_88_TCP_PORT
#               KDC address and port
#
#       SERVICE_PORT_80_TCP_ADDR
#       SERVICE_PORT_80_TCP_PORT
#               Http service address and port
#
#       KRB5_CONFIG_TEMPLATE
#       KRB5_CONFIG
#               The locations of the krb5.conf template, and where the
#               processed file must go
#
#       GSSAPI_PATH
#               The gssapi .so
#
#       TEST_DIR
#               The directory to build the client test app in
#
#       SERVICE_NAME
#       REALM_NAME
#       DOMAIN_NAME
#       USER_NAME
#       USER_PASSWORD

export PATH=$PATH:$GOROOT/bin

cat $KRB5_CONFIG_TEMPLATE \
        | sed -e "s/KDC_ADDRESS/$KDC_PORT_88_TCP_ADDR:$KDC_PORT_88_TCP_PORT/g" \
        | sed -e "s/DOMAIN_NAME/${DOMAIN_NAME}/g" \
        | sed -e "s/REALM_NAME/${REALM_NAME}/g" \
	> $KRB5_CONFIG

echo ${USER_PASSWORD} | kinit -V ${USER_NAME}@${REALM_NAME} >/dev/null

(cd $TEST_DIR && go test -c -o test -tags 'clienttest' github.com/apcera/gssapi/test)

# --test.bench=.
# --test.benchtime=2s
$TEST_DIR/test \
	--test.v=true \
        --debug=true \
	--service-name=${SERVICE_NAME} \
	--service-address=$SERVICE_PORT_80_TCP_ADDR:$SERVICE_PORT_80_TCP_PORT \
	--gssapi-path=$GSSAPI_PATH \
        2>&1
