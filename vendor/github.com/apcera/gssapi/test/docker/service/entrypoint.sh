#!/bin/bash -eu

export PATH=$PATH:$GOROOT/bin

cat /tmp/krb5.conf.template \
        | sed -e "s/KDC_ADDRESS/$KDC_PORT_88_TCP_ADDR:$KDC_PORT_88_TCP_PORT/g" \
        | sed -e "s/DOMAIN_NAME/${DOMAIN_NAME}/g" \
        | sed -e "s/REALM_NAME/${REALM_NAME}/g" \
	> /opt/go-gssapi-test-service/krb5.conf

(cd /opt/go-gssapi-test-service && go test -c -o test -tags 'servicetest' github.com/apcera/gssapi/test)

exec /opt/go-gssapi-test-service/test \
	--test.v=true \
        --debug=true \
        --service=true \
	--service-name=${SERVICE_NAME} \
	--service-address=:80 \
	--gssapi-path=/usr/lib/x86_64-linux-gnu/libgssapi_krb5.so.2 \
        --krb5-ktname=/opt/go-gssapi-test-service/krb5.keytab \
        --krb5-config=/opt/go-gssapi-test-service/krb5.conf \
        2>&1
