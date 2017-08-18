
# Setting up Service Catalog for API Aggregation in Kubernetes

The aggregator is a server that sits in front of the core API
Server. It allows API servers to come and go and register themselves
on demand to increase the API that kubernetes offers. Instead of
having multiple API servers on different ports, it allows them all to
be combined and aggregated together. This is good for several
reasons. It allows a client to only use a single API point. This
provides a much better user experience. It allows the server to be
secured once behind a single API point.

We need to provide a set of certificates to be provided as a
certificate bundle to the APIService apiregistration endpoint.

For development purposes, it is convenient to use the existing CA
automatically set up by the kubernetes development environment. The
[script in contrib](../contrib/svc-cat-apiserver-aggregation-tls-setup.sh)
generates a fresh CA and certificate setup, without using any existing
kubernetes infrastructure CAs or certificates. This script should be
`source`ed to define all of the variables it contains in the current
shell process.

The aggregator is a new feature of kubernetes and is an alpha API
running as a separate pod in kubernetes v1.6. The aggregator is
enabled by default in kubernetes v1.7 as a beta API, and is the
default secure endpoint directly integrated into the core kubernetes
APIServer.

For background on why we're messing with certificates and CA's at all,
please check [the auth doc](auth.md).

# Steps

## Prerequisites
You need `cfssl` tools, install with `go get -u github.com/cloudflare/cfssl/cmd/...`.

It is recommended to create a fresh directory, such as `certs/`, to
hold the generated config and certificates, before running the script.

## Check that aggregator is enabled

`kubectl api-versions` MUST list `apiregistration.k8s.io/v1beta1` (or
`apiregistration.k8s.io/v1alpha1` if running in kubernetes v1.6)

This API Group will not show up if you are talking to the insecure
port. The insecure port is not behind the aggregator. The aggregator
does not support routing of requests to the insecure port. You must
talk to the secure port, as the aggregator does not serve an API on
the insecure port.

## Variables used during the rest of the steps

These will be used during the steps and for the final `helm
install`. They are key to set, as the signed certificates will be
bound to the exact service name that is defined below as
`SVCCAT_SERVICE_NAME`. `SVCCAT_SERVICE_NAME` will be the exact DNS
entry that the generated certificate is bound to, so any deviation
from the use of these defined variables will result in a certificate
that is useless for the purposes of aggregation. All of the DNS
entries must match.

```
export HELM_RELEASE_NAME=catalog
export SVCCAT_NAMESPACE=catalog
export SVCCAT_SERVICE_NAME=${HELM_RELEASE_NAME}-catalog-apiserver
```

## Set up the certificate bundle

The APIService expects a certificate bundle. We can create our own, or
pull the one from kube core for reuse.

The certificate bundle is made up of Certificate Authority, a Serving
Certificate, and the Serving Private Key.

### Create our own new CA and generate keys

This is an example. It is written with zero understanding of the best
practices of secure certificate generation.

```
export CA_NAME=ca

export ALT_NAMES="\"${SVCCAT_SERVICE_NAME}.${SVCCAT_NAMESPACE}\",\"${SVCCAT_SERVICE_NAME}.${SVCCAT_NAMESPACE}.svc"\"

export SVCCAT_CA_SETUP=svc-cat-ca.json
cat >> ${SVCCAT_CA_SETUP} << EOF
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


cfssl genkey --initca ${SVCCAT_CA_SETUP} | cfssljson -bare ${CA_NAME}
# now the files 'ca.csr  ca-key.pem  ca.pem' exist

export SVCCAT_CA_CERT=${CA_NAME}.pem
export SVCCAT_CA_KEY=${CA_NAME}-key.pem

export PURPOSE=server
echo '{"signing":{"default":{"expiry":"43800h","usages":["signing","key encipherment","'${PURPOSE}'"]}}}' > "${PURPOSE}-ca-config.json"

echo '{"CN":"'${SVCCAT_SERVICE_NAME}'","hosts":['${ALT_NAMES}'],"key":{"algo":"rsa","size":2048}}' \
 | cfssl gencert -ca=${SVCCAT_CA_CERT} -ca-key=${SVCCAT_CA_KEY} -config=server-ca-config.json - \
 | cfssljson -bare apiserver

export SC_SERVING_CA=${SVCCAT_CA_CERT}
export SC_SERVING_CERT=apiserver.pem
export SC_SERVING_KEY=apiserver-key.pem
```

## Get the appropriate tls ca/cert/key from kube

If you are in a cloud provider environment, you most likely do not
have access to the appropriate keys.

The key we are looking for in an already running system is the
`--root-ca-file` flag to the controller-manager.

The following is an example based on a real kubernetes cluster. The
various files may be named differently and in different locations.


```
export SERVING_NAME=server-ca
export SERVINGCA_CERT=${SERVING_NAME}.crt
export SERVINGCA_KEY=${SERVING_NAME}.key
# a default location
cp /var/run/kubernetes/${SERVINGCA_CERT} .
cp /var/run/kubernetes/${SERVINGCA_KEY} .
```

### Create a cfssl config file for a new signing key

```
export PURPOSE=server
echo '{"signing":{"default":{"expiry":"43800h","usages":["signing","key encipherment","'${PURPOSE}'"]}}}' > "${PURPOSE}-ca-config.json"
```

### Use the existing keys plus the config file to generate the new signing certificate and key

```
export NAME_SPACE=catalog
export SERVICE_NAME=catalog-catalog-apiserver
export ALT_NAMES="\"${SERVICE_NAME}.${NAME_SPACE}\",\"${SERVICE_NAME}.${NAME_SPACE}.svc"\"
echo '{"CN":"'${SERVICE_NAME}'","hosts":['${ALT_NAMES}'],"key":{"algo":"rsa","size":2048}}' | cfssl gencert -ca=${SERVINGCA_CERT} -ca-key=${SERVINGCA_KEY} -config=server-ca-config.json - | cfssljson -bare apiserver
```

### Final Key Names

These variables define the final names of the resulting keys.

```
export SC_SERVING_CA=${SERVINGCA_CERT}
export SC_SERVING_CERT=apiserver.pem
export SC_SERVING_KEY=apiserver-key.pem
```

## Install the Service Catalog Chart with Helm

Use helm to install the Service Catalog, associating it with the
configured name ${HELM_NAME}, and into the specified namespace." This
command also enables authentication and aggregation and provides the
keys we just generated inline.

```
helm install charts/catalog \
    --name ${HELM_RELEASE_NAME} --namespace ${SVCCAT_NAMESPACE} \
    --set apiserver.auth.enabled=true \
        --set useAggregator=true \
        --set apiserver.tls.ca=$(base64 --wrap 0 ${SC_SERVING_CA}) \
        --set apiserver.tls.cert=$(base64 --wrap 0 ${SC_SERVING_CERT}) \
        --set apiserver.tls.key=$(base64 --wrap 0 ${SC_SERVING_KEY})
``` 

`servicecatalog.k8s.io/v1alpha1` should show up under `kubectl
api-versions` almost immediately, but kubectl will be slow to respond
to other commands until the apiserver is fully running.

If it doesn't show up the kubectl discovery cache is stale and needs
to be deleted. It may be located in the `.kube` directory,
approximately `~/.kube/cache/discovery/`.

Now Service Catalog e2e tests should work with the same `kubeconfig` set
for both the core APIServer access and the Service Catalog APIServer
access.

```
export SERVICECATALOGCONFIG=~/.kube/config
export KUBECONFIG=~/.kube/config
make test-e2e
```

# Summary

Before installing the helm chart, run the script in contrib by
`source`ing it, to define all of the necessary variables.

```shell
source /contrib/svc-cat-apiserver-aggregation-tls-setup.sh
```
