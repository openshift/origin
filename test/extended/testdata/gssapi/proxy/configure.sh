#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

cd /

# Get environment
export USER="$(whoami)"

# Edit kerberos config files, replacing EXAMPLE.COM with ${REALM}, and example.com with ${HOST}
for file in /etc/krb5.conf /var/kerberos/krb5kdc/kdc.conf /var/kerberos/krb5kdc/kadm5.acl; do
  sed -i.bak1 -e "s/EXAMPLE\.COM/${REALM}/g" $file
  sed -i.bak2 -e "s/example\.com/${HOST}/g" $file
done

# Create ticket database
kdb5_util create -s -r "${REALM}" -P password

# Add local user as admin
kadmin.local -q "addprinc -pw password ${USER}/admin@${REALM}"

# Start ticket server
krb5kdc

# Add user principal for current user, for test users user1-user5, the host principal (for ssh), the HTTP principal (for Apache), and create keytab
for u in "${USER}" user1 user2 user3 user4 user5; do
  kadmin.local -q "addprinc -pw password ${u}@${REALM}"
done

# Setup keytab for sshd
kadmin.local -q "addprinc -randkey host/${HOST}@${REALM}"
kadmin.local -q "ktadd -k /etc/krb5.keytab host/${HOST}@${REALM}"

# Setup keytab for apache
kadmin.local -q "addprinc -randkey HTTP/${HOST}@${REALM}"
kadmin.local -q "ktadd -k /etc/httpd.keytab HTTP/${HOST}@${REALM}"
chown apache /etc/httpd.keytab

# configure Apache proxy and auth
sed -i.bak1 -e "s#proxy\.example\.com#${HOST}#g" /etc/httpd/conf.d/proxy.conf
sed -i.bak2 -e "s#https://backend\.example\.com#${BACKEND}#g" /etc/httpd/conf.d/proxy.conf

# Start apache
httpd -k start

# Keep the service running
while true ; do sleep 60 ; done
