#!/bin/bash

CERT=${1:-}
KEY=${2:-}
P12=${3:-}
PASSWORD=${4:-}

if [ "${CERT}" == "" ] || [ "${KEY}" == "" ] || [ "${P12}" == "" ] || [ "${PASSWORD}" == "" ]; then
	echo "Usage: make-p12-cert.sh cert.crt key.key out.p12 password"
	exit 1
fi

openssl pkcs12 -export -inkey "${KEY}" -in "${CERT}" -out "${P12}" -password "pass:${PASSWORD}"
