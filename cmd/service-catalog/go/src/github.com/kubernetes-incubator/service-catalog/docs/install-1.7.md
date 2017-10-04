# Installing Service Catalog on Clusters Running Kubernetes 1.7 and Above

Kubernetes 1.7 or higher clusters run the
[API Aggregator](https://kubernetes.io/docs/concepts/api-extension/apiserver-aggregation/),
which is a specialized proxy server that sits in front of the core API Server.

The aggregator allows user-defined, Kubernetes compatible API servers to come
and go inside the cluster, and register themselves on demand to augment the
externally facing API that Kubernetes offers.

Instead of requiring the end-user to access multiple API servers, the API
aggregation system allows many API servers to run inside the cluster, and combines
all of their APIs into one externally facing API.

This system is very useful from an end-user's perspective, as it allows the
client to use a single API point with familiar, consistent tooling,
authentication and authorization.

The Service Catalog utilizes API aggregation to present its API.

# Step 1 - Prerequisites

## Starting Kubernetes with DNS

You *must* have a Kubernetes cluster with cluster DNS enabled. We can't list
instructions here for enabling cluster DNS for all Kubernetes cluster
installations, but here are a few notes:

* If you are using a cloud-based Kubernetes cluster or minikube, you likely
have cluster DNS enabled already.
* If you are using `hack/local-up-cluster.sh`, ensure the
`KUBE_ENABLE_CLUSTER_DNS` environment variable is set and then run the install
script

## Helm

You *must* use [Helm](http://helm.sh/) v2.5.0 or newer in the installation steps
below.

If you already have an appropriate Helm version, execute `helm init`
(if you haven't already) to install Tiller (the server-side component of Helm),
and you should be done with Helm setup.

If you don't already have an appropriate Helm version, see the
[Helm installation instructions](https://github.com/kubernetes/helm/blob/master/docs/install.md).

### Helm Charts

You need to download the
[charts/catalog](https://github.com/kubernetes-incubator/service-catalog/tree/master/charts/catalog)
directory to your local machine. Please refer to
[here](https://github.com/kubernetes-incubator/service-catalog/blob/master/docs/devguide.md#2-clone-fork-to-local-storage)
for the guide.

## RBAC

If your Kubernetes cluster must have RBAC enabled, then your Tiller pod needs to
have `cluster-admin` access.

If you are using Minikube, make sure to run your `minikube start` command with
this flag:

```console
minikube start --extra-config=apiserver.Authorization.Mode=RBAC
```
If you are using `hack/local-up-cluster.sh`, ensure the
`AUTHORIZATION_MODE` environment variable is set as follows:

```console
AUTHORIZATION_MODE=Node,RBAC hack/local-up-cluster.sh -O
```

By default, `helm init` installs the Tiller pod into the `kube-system`
namespace, with Tiller configured to use the `default` service account.

Configure Tiller with `cluster-admin` access with the following command:

```console
kubectl create clusterrolebinding tiller-cluster-admin \
    --clusterrole=cluster-admin \
    --serviceaccount=kube-system:default
```

If you used the `--tiller-namespace` or `--service-account` flags when running
`helm init`, the `--serviceaccount` flag in the previous command needs to be
adjusted to reference the appropriate namespace and ServiceAccount name.

## A Recent `kubectl`

As with Kubernetes itself, interaction with the service catalog system is
achieved through the `kubectl` command line interface. Chances are high that
you already have this installed, however, the service catalog *requires*
`kubectl` version 1.6 or newer.

To proceed, we must:

- Download and install `kubectl` version 1.6 or newer.
- Configure `kubectl` to communicate with the service catalog's API server.

To install `kubectl` follow the [standard instructions](https://kubernetes.io/docs/tasks/kubectl/install/).

For example, on a mac,
```console
curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/darwin/amd64/kubectl
chmod +x ./kubectl
```

We'll assume that all `kubectl` commands are using this newly-installed
executable.

# Step 2 - Generate TLS Certificates

We provide a script to do all of the steps needed to set up TLS certificates
that the aggregation system uses. If you'd like to read how to do this setup
manually, please see the
[manual API aggregation setup document](./manual-api-aggregation-setup.md).

Otherwise, read on for automated instructions.

First, create a directory in which certificates will be generated:

```console
mkdir certs
cd certs
```

We'll assume that you're operating from this `docs/certs` directory for the
remainder of this document.

Next, install the `cfssl` toolchain (which the following script uses):

```console
go get -u github.com/cloudflare/cfssl/cmd/...
```

Finally, create the certs:

```console
source ../../contrib/svc-cat-apiserver-aggregation-tls-setup.sh
```

# Step 3 - Install Service Catalog with Helm Chart

Use helm to install the Service Catalog, associating it with the
configured name ${HELM_RELEASE_NAME}, and into the specified namespace." This
command also enables authentication and authorization and provides the
keys we just generated inline.

The installation commands vary slightly between Linux and Mac OS X because of
the versions of the `base64` command (Linux has GNU base64, Mac OS X has BSD
base64). If you're installing from a Linux based machine, run this:

```
helm install ../../charts/catalog \
    --name ${HELM_RELEASE_NAME} --namespace ${SVCCAT_NAMESPACE} \
    --set apiserver.auth.enabled=true \
        --set useAggregator=true \
        --set apiserver.tls.ca=$(base64 --wrap 0 ${SC_SERVING_CA}) \
        --set apiserver.tls.cert=$(base64 --wrap 0 ${SC_SERVING_CERT}) \
        --set apiserver.tls.key=$(base64 --wrap 0 ${SC_SERVING_KEY})
```

If you're on a Mac OS X based machine, run this:

```
helm install ../../charts/catalog \
    --name ${HELM_RELEASE_NAME} --namespace ${SVCCAT_NAMESPACE} \
    --set apiserver.auth.enabled=true \
        --set useAggregator=true \
        --set apiserver.tls.ca=$(base64 ${SC_SERVING_CA}) \
        --set apiserver.tls.cert=$(base64 ${SC_SERVING_CERT}) \
        --set apiserver.tls.key=$(base64 ${SC_SERVING_KEY})
```
