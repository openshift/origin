#!/bin/bash -eu

# Add kerberos principals.
echo -e "\n\n\n\n\n\n${USER_PASSWORD}\n${USER_PASSWORD}\n" | kadmin -l add ${SERVICE_NAME}@${REALM_NAME}
echo -e "\n\n\n\n\n\n${USER_PASSWORD}\n${USER_PASSWORD}\n" | kadmin -l add ${USER_NAME}@${REALM_NAME}
#kadmin -l list --long ${USER_NAME}@${REALM_NAME}
#kadmin -l list --long ${SERVICE_NAME}@${REALM_NAME}

# Export keytab.
kadmin -l ext_keytab -k /etc/docker-kdc/krb5.keytab ${SERVICE_NAME}@${REALM_NAME}

# KDC daemon startup.
#TODO -- what's relevant in this config? Need to provide my own?
exec /usr/lib/heimdal-servers/kdc --config-file=/etc/heimdal-kdc/kdc.conf -P 88


