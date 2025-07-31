#!/usr/bin/env bash
set -eou pipefail

WORKING_DIR=${1:-/tmp/_working_dir}
NAMESPACE=${2:-openshift-logging}
CA_PATH=$WORKING_DIR/ca
cn_name="aosqeca"
CLIENT_CA_PATH="${WORKING_DIR}/client"
CLUSTER_CA_PATH="${WORKING_DIR}/cluster"
REGENERATE_NEEDED=0

function init_cert_files() {
if [ ! -d $CA_PATH ]; then
    mkdir $CA_PATH
fi
if [ ! -d $CLIENT_CA_PATH ]; then
    mkdir ${CLIENT_CA_PATH}
fi
if [ ! -d $CLUSTER_CA_PATH ]; then
    mkdir ${CLUSTER_CA_PATH}
fi
}

function create_root_ca() {
#fqdn=my-cluster-kafka-bootstrap.amq-aosqe.svc
#1)  generate root ca key and and self cert
openssl req -x509 -new -newkey rsa:4096 -keyout $CA_PATH/root_ca.key -nodes -days 1825 -out $CA_PATH/ca_bundle.crt -subj "/CN=aosqeroot"   -passin pass:aosqe2021 -passout pass:aosqe2021

# Create trustore jks
/usr/lib/jvm/jre-openjdk/bin/keytool -import -file $CA_PATH/ca_bundle.crt -keystore $CA_PATH/ca_bundle.jks  --srcstorepass aosqe2021 --deststorepass aosqe2021 -noprompt || exit 1
}

# Create Client CSR
function create_client_sign() {
cat <<EOF >$CLIENT_CA_PATH/client_csr.conf
[ req ]
default_bits = 4096
prompt = no
encrypt_key = yes
default_md = sha512
distinguished_name = dn
req_extensions = server_ext

[ dn ]
CN = "aosqeclient"

[ server_ext ]
basicConstraints        = CA:false
extendedKeyUsage        = serverAuth,clientAuth
subjectAltName = DNS.1:*.cluster.local,DNS.2:*.svc,DNS.3:*.pod
EOF
openssl req -new -out $CLIENT_CA_PATH/client.csr -newkey rsa:4096 -keyout $CLIENT_CA_PATH/client.key -config $CLIENT_CA_PATH/client_csr.conf -nodes


# Sign client ca by intermediate ca
cat <<EOF >$CLIENT_CA_PATH/client_sign.conf
# Simple Signing CA

# The [default] section contains global constants that can be referred to from
# the entire configuration file. It may also hold settings pertaining to more
# than one openssl command.

[ default ]
dir                     = $CLIENT_CA_PATH             # Top dir

# The remainder of the configuration file is used by the openssl ca command.
# The CA section defines the locations of CA assets, as well as the policies
# applying to the CA.

[ ca ]
default_ca              = signing_ca            # The default CA section

[ signing_ca ]
certificate             = $CA_PATH/ca_bundle.crt       # The CA cert
private_key             = $CA_PATH/root_ca.key # CA private key
new_certs_dir           = $CA_PATH/           # Certificate archive
serial                  = $CA_PATH/root_ca.serial.txt # Serial number file
crlnumber               = $CA_PATH/root_ca.crl.srl # CRL number file
database                = $CA_PATH/root_ca.db # Index file
unique_subject          = no                    # Require unique subject
default_days            = 730                   # How long to certify for
default_md              = sha512                # MD to use
policy                  = any_pol             # Default naming policy
email_in_dn             = no                    # Add email to cert DN
preserve                = no                    # Keep passed DN ordering
name_opt                = ca_default            # Subject DN display options
cert_opt                = ca_default            # Certificate display options
copy_extensions         = copy                  # Copy extensions from CSR
x509_extensions         = server_ext             # Default cert extensions
default_crl_days        = 7                     # How long before next CRL

# Naming policies control which parts of a DN end up in the certificate and
# under what circumstances certification should be denied.

[ any_pol ]
domainComponent         = optional
countryName             = optional
stateOrProvinceName     = optional
localityName            = optional
organizationName        = optional
organizationalUnitName  = optional
commonName              = optional
emailAddress            = optional

# Certificate extensions define what types of certificates the CA is able to
# create.

[ server_ext ]
basicConstraints        = CA:false
extendedKeyUsage        = serverAuth,clientAuth

[ ca_reqext ]
basicConstraints        = CA:false


# CRL extensions exist solely to point to the CA certificate that has issued
# the CRL.
EOF

touch $CA_PATH/root_ca.db
if [ ! -f $CA_PATH/root_ca.serial.txt ] ; then
    echo "01">$CA_PATH/root_ca.serial.txt
fi
openssl ca -in $CLIENT_CA_PATH/client.csr -notext -out $CLIENT_CA_PATH/client.crt -config $CLIENT_CA_PATH/client_sign.conf -batch

# Create Client keystone
openssl pkcs12 -export -in $CLIENT_CA_PATH/client.crt -inkey $CLIENT_CA_PATH/client.key -out $CLIENT_CA_PATH/client.pkcs12  -passin pass:aosqe2021 -passout pass:aosqe2021
/usr/lib/jvm/jre-openjdk/bin/keytool -importkeystore -srckeystore $CLIENT_CA_PATH/client.pkcs12 -srcstoretype PKCS12 -destkeystore $CLIENT_CA_PATH/client.jks -deststoretype JKS --srcstorepass aosqe2021 --deststorepass aosqe2021 -noprompt
}

# Create cluster csr
# https://support.dnsimple.com/articles/ssl-certificate-names/
function create_cluster_sign() {
cat <<EOF >$CLUSTER_CA_PATH/cluster_csr.conf
[ req ]
default_bits = 4096
prompt = no
encrypt_key = yes
default_md = sha512
distinguished_name = dn
req_extensions = server_ext

[ dn ]
CN = "aosqecluster"

[ server_ext ]
basicConstraints        = CA:false
extendedKeyUsage        = serverAuth,clientAuth
subjectAltName = DNS.1:kafka.${NAMESPACE}.svc.cluster.local,DNS.2:kafka.${NAMESPACE}.svc,DNS.3:kafka-0.kafka.${NAMESPACE}.svc.cluster.local,DNS.4: kafka-0.kafka.${NAMESPACE}.svc, DNS.5: kafka, DNS.6: kakfa-0
EOF
openssl req -new -out $CLUSTER_CA_PATH/cluster.csr -newkey rsa:4096 -keyout $CLUSTER_CA_PATH/cluster.key -config $CLUSTER_CA_PATH/cluster_csr.conf -nodes


cat <<EOF >$CLUSTER_CA_PATH/cluster_sign.conf
# Simple Signing CA

# The [default] section contains global constants that can be referred to from
# the entire configuration file. It may also hold settings pertaining to more
# than one openssl command.

[ default ]
dir                     = $CA_PATH             # Top dir

# The next part of the configuration file is used by the openssl req command.
# It defines the CA's key pair, its DN, and the desired extensions for the CA
# certificate.

[ req ]
default_bits            = 4096                  # RSA key size
encrypt_key             = yes                   # Protect private key
default_md              = sha512                # MD to use
utf8                    = yes                   # Input is UTF-8
string_mask             = utf8only              # Emit UTF-8 strings
prompt                  = no                    # Don't prompt for DN
distinguished_name      = ca_dn                 # DN section
req_extensions          = ca_reqext             # Desired extensions

[ ca_dn ]
commonName              = "aosqeintermediate"

[ ca_reqext ]
basicConstraints        = CA:false

# The remainder of the configuration file is used by the openssl ca command.
# The CA section defines the locations of CA assets, as well as the policies
# applying to the CA.

[ ca ]
default_ca              = signing_ca            # The default CA section

[ signing_ca ]
certificate             = $CA_PATH/ca_bundle.crt       # The CA cert
private_key             = $CA_PATH/root_ca.key # CA private key
new_certs_dir           = $CA_PATH/           # Certificate archive
serial                  = $CA_PATH/root_ca.serial.txt # Serial number file
crlnumber               = $CA_PATH/root_ca.crl.srl # CRL number file
database                = $CA_PATH/root_ca.db # Index file
unique_subject          = no                    # Require unique subject
default_days            = 730                   # How long to certify for
default_md              = sha512                # MD to use
policy                  = any_pol             # Default naming policy
email_in_dn             = no                    # Add email to cert DN
preserve                = no                    # Keep passed DN ordering
name_opt                = ca_default            # Subject DN display options
cert_opt                = ca_default            # Certificate display options
copy_extensions         = copy                  # Copy extensions from CSR
#x509_extensions         = server_ext             # Default cert extensions
default_crl_days        = 7                     # How long before next CRL

# Naming policies control which parts of a DN end up in the certificate and
# under what circumstances certification should be denied.

[ match_pol ]
domainComponent         = match                 # Must match 'simple.org'
organizationName        = match                 # Must match 'Simple Inc'
organizationalUnitName  = optional              # Included if present
commonName              = supplied              # Must be present

[ any_pol ]
domainComponent         = optional
countryName             = optional
stateOrProvinceName     = optional
localityName            = optional
organizationName        = optional
organizationalUnitName  = optional
commonName              = optional
emailAddress            = optional

# Certificate extensions define what types of certificates the CA is able to
# create.

[ server_ext ]
basicConstraints        = CA:false
extendedKeyUsage        = serverAuth,clientAuth
subjectAltName = DNS.1:kafka.${NAMESPACE}.svc.cluster.local,DNS.2:kafka.${NAMESPACE}.svc,DNS.3:kafka-0.kafka.${NAMESPACE}.svc.cluster.local,DNS.4: kafka-0.kafka.${NAMESPACE}.svc, DNS.5: kafka, DNS.6: kakfa-0
EOF

touch $CA_PATH/root_ca.db
if [ ! -f $CA_PATH/root_ca.serial.txt ] ; then
    echo "01">$CA_PATH/root_ca.serial.txt
fi
openssl ca -in $CLUSTER_CA_PATH/cluster.csr -notext -out $CLUSTER_CA_PATH/cluster.crt -config $CLUSTER_CA_PATH/cluster_sign.conf -batch

#Create keystone
openssl pkcs12 -export -in $CLUSTER_CA_PATH/cluster.crt -inkey $CLUSTER_CA_PATH/cluster.key -out $CLUSTER_CA_PATH/cluster.pkcs12  -passin pass:aosqe2021 -passout pass:aosqe2021
/usr/lib/jvm/jre-openjdk/bin/keytool -importkeystore -srckeystore $CLUSTER_CA_PATH/cluster.pkcs12 -srcstoretype PKCS12 -destkeystore $CLUSTER_CA_PATH/cluster.jks -deststoretype JKS  --srcstorepass aosqe2021 --deststorepass aosqe2021 -noprompt
}

init_cert_files
create_root_ca
create_client_sign
create_cluster_sign
