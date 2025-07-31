#!/bin/bash

WORKING_DIR=$1
NAMESPACE=$2
CA_PATH=${CA_PATH:-$WORKING_DIR/ca.crt}
LOG_STORE=$3
REGENERATE_NEEDED=0

function info() {
  local msg=$1
  local type=${2:-}
  local reason=${3:-}
  local out="\"msg\":\"$msg\""
  if [ -n "$type" ] ; then
    out="$out, \"type\":\"$type\""
  fi
  if [ -n "$reason" ] ; then
    out="$out, \"reason\":\"$reason\""
  fi
  if [ -f ${WORKING_DIR}/action.log ] && [ "$(stat -c%s ${WORKING_DIR}/action.log)" != "0" ] ; then
    echo "," >> ${WORKING_DIR}/action.log
  fi
  echo "{$out}" >> ${WORKING_DIR}/action.log
}

function init_cert_files() {

  if [ ! -f ${WORKING_DIR}/ca.db ]; then
    info "${WORKING_DIR}/ca.db" "File intialization" "Missing"
    touch ${WORKING_DIR}/ca.db
  fi

  if [ ! -f ${WORKING_DIR}/ca.serial.txt ]; then
    info "${WORKING_DIR}/ca.serial.txt" "File intialization" "Missing"
    echo 00 > ${WORKING_DIR}/ca.serial.txt
  fi
}

function generate_signing_ca() {
  local regen=false
  local fileExists=$(test -f ${WORKING_DIR}/ca.crt;echo $?)
  if [ "$fileExists" == "1" ] ; then
    info "${WORKING_DIR}/ca.crt" "Regenerate" "FileMissing"
    regen=true
  fi
  local filetype=$(file -b ${WORKING_DIR}/ca.crt)
  if [ "$fileExists" == "0" ] && [ "$filetype" != "PEM certificate" ] ; then
    info "${WORKING_DIR}/ca.crt '$filetype' != 'PEM certificate'" "Regenerate" "InvalidFileType"
    regen=true
  fi
  fileExists=$(test -f ${WORKING_DIR}/ca.key;echo $?)
  if [ "$fileExists" == "1" ] ; then
    info "${WORKING_DIR}/ca.key" "Regenerate" "FileMissing"
    regen=true
  fi
  filetype=$(file -b ${WORKING_DIR}/ca.key)
  if [ "$fileExists" == "0" ] && [ "$filetype" != "ASCII text" ] ; then
    info "${WORKING_DIR}/ca.key '$filetype' != 'ASCII text'" "Regenerate" "InvalidFileType"
    regen=true
  fi
  if ! $(openssl x509 -checkend 0 -noout -in ${WORKING_DIR}/ca.crt > /dev/null 2>&1) ; then
    info "${WORKING_DIR}/ca.crt" "Regenerate" "ExpiredOrMissing"
    regen=true
  fi
  if [ "${regen}" == "true" ] ; then
    info "generating signing request"
    openssl req -x509 \
                -new \
                -newkey rsa:4096 \
                -keyout ${WORKING_DIR}/ca.key \
                -nodes \
                -days 1825 \
                -out ${WORKING_DIR}/ca.crt \
                -subj "/CN=openshift-cluster-logging-signer"

    REGENERATE_NEEDED=1
  fi
}

function create_signing_conf() {
  cat <<EOF > "${WORKING_DIR}/signing.conf"
# Simple Signing CA

# The [default] section contains global constants that can be referred to from
# the entire configuration file. It may also hold settings pertaining to more
# than one openssl command.

[ default ]
dir                     = ${WORKING_DIR}               # Top dir

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
0.domainComponent       = "io"
1.domainComponent       = "openshift"
organizationName        = "OpenShift Origin"
organizationalUnitName  = "Logging Signing CA"
commonName              = "Logging Signing CA"

[ ca_reqext ]
keyUsage                = critical,keyCertSign,cRLSign
basicConstraints        = critical,CA:true,pathlen:0
subjectKeyIdentifier    = hash

# The remainder of the configuration file is used by the openssl ca command.
# The CA section defines the locations of CA assets, as well as the policies
# applying to the CA.

[ ca ]
default_ca              = signing_ca            # The default CA section

[ signing_ca ]
certificate             = \$dir/ca.crt       # The CA cert
private_key             = \$dir/ca.key # CA private key
new_certs_dir           = \$dir/           # Certificate archive
serial                  = \$dir/ca.serial.txt # Serial number file
crlnumber               = \$dir/ca.crl.srl # CRL number file
database                = \$dir/ca.db # Index file
unique_subject          = no                    # Require unique subject
default_days            = 730                   # How long to certify for
default_md              = sha512                # MD to use
policy                  = any_pol             # Default naming policy
email_in_dn             = no                    # Add email to cert DN
preserve                = no                    # Keep passed DN ordering
name_opt                = ca_default            # Subject DN display options
cert_opt                = ca_default            # Certificate display options
copy_extensions         = copy                  # Copy extensions from CSR
x509_extensions         = client_ext             # Default cert extensions
default_crl_days        = 7                     # How long before next CRL
crl_extensions          = crl_ext               # CRL extensions

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

[ client_ext ]
keyUsage                = critical,digitalSignature,keyEncipherment
basicConstraints        = CA:false
extendedKeyUsage        = clientAuth
subjectKeyIdentifier    = hash
authorityKeyIdentifier  = keyid

[ server_ext ]
keyUsage                = critical,digitalSignature,keyEncipherment
basicConstraints        = CA:false
extendedKeyUsage        = serverAuth,clientAuth
subjectKeyIdentifier    = hash
authorityKeyIdentifier  = keyid

# CRL extensions exist solely to point to the CA certificate that has issued
# the CRL.

[ crl_ext ]
authorityKeyIdentifier  = keyid
EOF
}

function sign_cert() {
  local component=$1
  info "Signing cert for component: $component"
  openssl ca \
          -in ${WORKING_DIR}/${component}.csr  \
          -notext                              \
          -out ${WORKING_DIR}/${component}.crt \
          -config ${WORKING_DIR}/signing.conf  \
          -extensions v3_req                   \
          -batch                               \
          -extensions server_ext
}

function generate_cert_config() {
  local component=$1
  local extensions=${2:-}
  info "generating cert config for component '$component' with ext: '${extensions}'"
  if [ "$extensions" != "" ]; then
    cat <<EOF > "${WORKING_DIR}/${component}.conf"
[ req ]
default_bits = 4096
prompt = no
encrypt_key = yes
default_md = sha512
distinguished_name = dn
req_extensions = req_ext
[ dn ]
CN = ${component}
OU = OpenShift
O = Logging
[ req_ext ]
subjectAltName = ${extensions}
EOF
  else
    cat <<EOF > "${WORKING_DIR}/${component}.conf"
[ req ]
default_bits = 4096
prompt = no
encrypt_key = yes
default_md = sha512
distinguished_name = dn
[ dn ]
CN = ${component}
OU = OpenShift
O = Logging
EOF
  fi
}

function generate_request() {
  local component=$1
  info "signing request for component: $component"
  openssl req -new                                        \
          -out ${WORKING_DIR}/${component}.csr            \
          -newkey rsa:4096                                \
          -keyout ${WORKING_DIR}/${component}.key         \
          -config ${WORKING_DIR}/${component}.conf        \
          -days 712                                       \
          -nodes
}

function generate_certs() {
  local component=$1
  local extensions=${2:-}
  local regen=false
  if [ $REGENERATE_NEEDED = 1 ] ; then
    info "REGENERATE_NEEDED=1"
    regen=true
  fi
  local fileExists=$(test -f ${WORKING_DIR}/${component}.crt;echo $?)
  if [ "$fileExists" == "1" ] ; then
    info "${WORKING_DIR}/${component}.crt" "Regenerate" "FileMissing"
    regen=true
  fi
  local filetype=$(file -b ${WORKING_DIR}/${component}.crt)
  if [ "$fileExists" == "0" ] && [ "$filetype" != "PEM certificate" ] ; then
    info "${WORKING_DIR}/${component}.crt '$filetype' != 'PEM certificate'" "Regenerate" "InvalidFileType"
    regen=true
  fi
  if ! $(openssl x509 -checkend 0 -noout -in ${WORKING_DIR}/${component}.crt > /dev/null 2>&1); then
    info "${WORKING_DIR}/${component}.crt" "Regenerate" "ExpiredOrMissing"
    regen=true
  fi
  if [ "${regen}" == "true" ]; then
    info "Generating certs for ${component} with ext: ${extensions}"
    generate_cert_config $component $extensions
    generate_request $component
    sign_cert $component
  fi
}

function generate_extensions() {
  local add_oid=$1
  local add_localhost=$2
  shift
  shift
  local cert_names=$@

  extension_names=""
  extension_index=1
  local use_comma=0

  if [ "$add_localhost" == "true" ]; then
    extension_names="IP.1:127.0.0.1,IP.2:0:0:0:0:0:0:0:1,DNS.1:localhost"
    extension_index=2
    use_comma=1
  fi

  for name in ${cert_names//,/}; do
    if [ $use_comma = 1 ]; then
      extension_names="${extension_names},DNS.${extension_index}:${name}"
    else
      extension_names="DNS.${extension_index}:${name}"
      use_comma=1
    fi
    extension_index=$(( extension_index + 1 ))
  done

  if [ "$add_oid" == "true" ]; then
    extension_names="${extension_names},RID.1:1.2.3.4.5.5"
  fi

  echo "$extension_names"
}

if [ ! -d $WORKING_DIR ]; then
  mkdir -p $WORKING_DIR
fi

generate_signing_ca
init_cert_files
create_signing_conf

generate_certs 'system.logging.fluentd'
generate_certs 'system.logging.kibana'
generate_certs 'system.admin'

# TODO: get es SAN DNS, IP values from es service names
generate_certs 'kibana-internal' "$(generate_extensions false false kibana)"
generate_certs 'elasticsearch' "$(generate_extensions true true $LOG_STORE{,-cluster}{,.${NAMESPACE}.svc})"
generate_certs 'logging-es' "$(generate_extensions false true $LOG_STORE{,.${NAMESPACE}.svc})"

if [ ! -s "${WORKING_DIR}/kibana-session-secret" ] ; then
  info "Generating kibana session secret"
  head -c 16 < /dev/urandom | hexdump -v -e '1/1 "%02X"' > "${WORKING_DIR}/kibana-session-secret"
fi

if [ -f ${WORKING_DIR}/action.log ] ; then
  # remove newlines from log to have condensed JSON array
  sed -i ':a;N;$!ba;s/\n//g' ${WORKING_DIR}/action.log
  echo "[$(cat ${WORKING_DIR}/action.log)]"
  rm ${WORKING_DIR}/action.log
fi
