# Manual API Aggregation Setup

This document describes how to manually set up artifacts that the helm chart
for Service Catalog needs to integrate with the API aggregator.

This repository provides a script to automatically set those artifacts up, and 
we recommend that you use it. If you'd like to do so, please see the
[install document](./install-1.7.md).

# Step 1 - Create TLS Certificates

We need to provide a set of certificates to be provided as a
certificate bundle to the `APIService` resource.

For development purposes, it is convenient to use the existing CA
automatically set up by the kubernetes development environment. The
[script in contrib](../contrib/svc-cat-apiserver-aggregation-tls-setup.sh)
generates a fresh CA and certificate setup, without using any existing
kubernetes infrastructure CAs or certificates. This script should be
`source`ed to define all of the variables it contains in the current
shell process.

For background on why we need to deal with certificates and CA's at all, you 
may read the [the auth doc](auth.md) (this is not required, however).

## Install `cfssl` Tools

Before we continue, you'll need to install the `cfssl` tools:

```console
go get -u github.com/cloudflare/cfssl/cmd/...
```

## Create a Certificates Directory

Please create a fresh directory called `certs` to hold the configuration
and certificates that we'll generate in the following steps:

```console
mkdir certs
cd certs
```

## Check That the API Aggregator is Enabled

Run the following:

```console
kubectl api-versions
```

This endpoint must list `apiregistration.k8s.io/v1beta1`. If it does not, your 
Kubernetes installation is likely version 1.6 or previous. We recommend running
1.7 or later, but if you decide to continue, please see the 
[installation document for Kubernetes 1.6](./install-1.6.md).

This `apiregistration.k8s.io/v1beta1` API Group will not show up if you are 
connecting to the insecure port of the core API server, so be sure that your
`kubectl` configuration file (often located at `~/.kube/config`) is pointing
to an https endpoint.

## Create Environment Variables

The following environment variables will be used during the following steps.
They will be passed to the certificate generation tools as well as the final
`helm install` step.

They are important to set, as the signed certificates will be bound to the 
exact service name that is defined below as `SVCCAT_SERVICE_NAME`. 

`SVCCAT_SERVICE_NAME` will be the exact DNS entry that the generated 
certificate is bound to, so any deviation from the use of these defined 
variables will result in a certificate that is useless for the purposes of 
aggregation.

```
export HELM_RELEASE_NAME=catalog
export SVCCAT_NAMESPACE=catalog
export SVCCAT_SERVICE_NAME=${HELM_RELEASE_NAME}-catalog-apiserver
```

## Get a Certificate Authority (CA) and Keys

There are two options to get a CA and keys.

### Option 1 - Create Our Own Certificate Authority and Generate Keys

The `APIService` resource expects a certificate bundle. We can create our own, 
or pull the one core Kubernetes API server for reuse.

The certificate bundle is made up of a Certificate Authority (CA), a Serving
Certificate, and the Serving Private Key. 

Run the following to create a CA and generate keys:

```console
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

### Options 2 - Get the Appropriate TLS CA, Certificate and Key from Kubernetes

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

## Create a cfssl Config File For a New Signing Key

```
export PURPOSE=server
echo '{"signing":{"default":{"expiry":"43800h","usages":["signing","key encipherment","'${PURPOSE}'"]}}}' > "${PURPOSE}-ca-config.json"
```

## Use the Existing Keys and the Config File to Generate the New Signing Certificate and Key

```
export NAME_SPACE=catalog
export SERVICE_NAME=catalog-catalog-apiserver
export ALT_NAMES="\"${SERVICE_NAME}.${NAME_SPACE}\",\"${SERVICE_NAME}.${NAME_SPACE}.svc"\"
echo '{"CN":"'${SERVICE_NAME}'","hosts":['${ALT_NAMES}'],"key":{"algo":"rsa","size":2048}}' | cfssl gencert -ca=${SERVINGCA_CERT} -ca-key=${SERVINGCA_KEY} -config=server-ca-config.json - | cfssljson -bare apiserver
```

## Final Key Names

These variables define the final names of the resulting keys.

```
export SC_SERVING_CA=${SERVINGCA_CERT}
export SC_SERVING_CERT=apiserver.pem
export SC_SERVING_KEY=apiserver-key.pem
```

# Step 2 - Install the Service Catalog Chart with Helm

Use helm to install the Service Catalog, associating it with the
configured name ${HELM_NAME}, and into the specified namespace." This
command also enables authentication and aggregation and provides the
keys we just generated inline.

The installation commands vary slightly between Linux and Mac OS X because of
the versions of the `base64` command (Linux has GNU base64, Mac OS X has BSD 
base64). If you're installing from a Linux based machine, run this:

```
helm install charts/catalog \
    --name ${HELM_RELEASE_NAME} --namespace ${SVCCAT_NAMESPACE} \
    --set apiserver.auth.enabled=true \
        --set useAggregator=true \
        --set apiserver.tls.ca=$(base64 --wrap 0 ${SC_SERVING_CA}) \
        --set apiserver.tls.cert=$(base64 --wrap 0 ${SC_SERVING_CERT}) \
        --set apiserver.tls.key=$(base64 --wrap 0 ${SC_SERVING_KEY})
```

If you're on a Mac OS X based machine, run this:

```
helm install charts/catalog \
    --name ${HELM_RELEASE_NAME} --namespace ${SVCCAT_NAMESPACE} \
    --set apiserver.auth.enabled=true \
        --set useAggregator=true \
        --set apiserver.tls.ca=$(base64 ${SC_SERVING_CA}) \
        --set apiserver.tls.cert=$(base64 ${SC_SERVING_CERT}) \
        --set apiserver.tls.key=$(base64 ${SC_SERVING_KEY})
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
