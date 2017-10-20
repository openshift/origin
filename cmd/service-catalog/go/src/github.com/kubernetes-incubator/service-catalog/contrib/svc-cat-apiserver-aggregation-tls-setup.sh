#!/bin/sh --

# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


export HELM_RELEASE_NAME=${HELM_RELEASE_NAME:-catalog}
export SVCCAT_NAMESPACE=${SVCCAT_NAMESPACE:-catalog}
SVCCAT_SERVICE_NAME=${HELM_RELEASE_NAME}-catalog-apiserver

# The location to create the output files may be customized by setting your
# CERT_FOLDER environment variable to the desired output folder. Leaving this
# blank will create them in the current directory.
#
# NOTE: Must contain a trailing slash. (e.g., CERT_FOLDER="/tmp/certs/")
CERT_FOLDER=${CERT_FOLDER:-}

CA_NAME=ca

ALT_NAMES="\"${SVCCAT_SERVICE_NAME}.${SVCCAT_NAMESPACE}\",\"${SVCCAT_SERVICE_NAME}.${SVCCAT_NAMESPACE}.svc"\"

SVCCAT_CA_SETUP=${CERT_FOLDER}svc-cat-ca.json
cat > ${SVCCAT_CA_SETUP} << EOF
{
    "hosts": [ ${ALT_NAMES} ],
    "key": {
        "algo": "rsa",
        "size": 2048
    },
    "names": [
        {
            "C": "US",
            "L": "san jose",
            "O": "kube",
            "OU": "WWW",
            "ST": "California"
        }
    ]
}
EOF


cfssl genkey --initca ${SVCCAT_CA_SETUP} | cfssljson -bare "${CERT_FOLDER}${CA_NAME}"
# now the files 'ca.csr  ca-key.pem  ca.pem' exist

SVCCAT_CA_CERT="${CERT_FOLDER}${CA_NAME}.pem"
SVCCAT_CA_KEY="${CERT_FOLDER}${CA_NAME}-key.pem"

PURPOSE=server
SVCCAT_SERVER_CA_CONFIG="${CERT_FOLDER}${PURPOSE}-ca-config.json"
echo '{"signing":{"default":{"expiry":"43800h","usages":["signing","key encipherment","'${PURPOSE}'"]}}}' > "${SVCCAT_SERVER_CA_CONFIG}"

echo '{"CN":"'${SVCCAT_SERVICE_NAME}'","hosts":['${ALT_NAMES}'],"key":{"algo":"rsa","size":2048}}' \
 | cfssl gencert -ca=${SVCCAT_CA_CERT} -ca-key=${SVCCAT_CA_KEY} -config=${SVCCAT_SERVER_CA_CONFIG} - \
 | cfssljson -bare "${CERT_FOLDER}apiserver"

export SC_SERVING_CA=${SVCCAT_CA_CERT}
export SC_SERVING_CERT=${CERT_FOLDER}apiserver.pem
export SC_SERVING_KEY=${CERT_FOLDER}apiserver-key.pem
