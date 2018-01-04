# Installing Service Catalog

Kubernetes 1.7 or higher clusters run the
[API Aggregator](https://kubernetes.io/docs/concepts/api-extension/apiserver-aggregation/),
which is a specialized proxy server that sits in front of the core API Server.

The aggregator allows user-defined, Kubernetes compatible API servers to come
and go inside the cluster, and register themselves on demand to augment the
externally facing API that Kubernetes offers.

Instead of requiring the end-user to access multiple API servers, the API
aggregation system allows many API servers to run inside the cluster, and
combines all of their APIs into one externally facing API.

This system is very useful from an end-user's perspective, as it allows the
client to use a single API endpoint with familiar, consistent tooling,
authentication and authorization.

The Service Catalog utilizes API aggregation to present its API.

# Step 1 - Prerequisites

## Starting Kubernetes with DNS

You *must* have a Kubernetes cluster with cluster DNS enabled. We can't list
instructions here for enabling cluster DNS for all Kubernetes distributions, but
here are a few notes:

* If you are using a cloud-based Kubernetes cluster or minikube, you likely have
cluster DNS enabled already.
* If you are using `hack/local-up-cluster.sh`, ensure the
`KUBE_ENABLE_CLUSTER_DNS` environment variable is set, then run the install
script.

## Helm

You *must* use [Helm](http://helm.sh/) v2.7.0 or newer in the installation
steps below.

If you already have an appropriate Helm version, execute `helm init` (if you
haven't already) to install Tiller (the server-side component of Helm), and you
should be done with Helm setup.

If you don't already have an appropriate Helm version, see the
[Helm installation instructions](https://github.com/kubernetes/helm/blob/master/docs/install.md).

### Helm Charts

You need to add the service-catalog Helm repository to your local machine.
Execute the following to do so:

```console
helm repo add svc-cat https://svc-catalog-charts.storage.googleapis.com
```

To ensure that it worked, execute the following:

```console
helm search service-catalog
```

You should see the following output:

```console
NAME           	VERSION	DESCRIPTION
svc-cat/catalog	0.0.1  	service-catalog API server and controller-manag...
```

## RBAC

Your Kubernetes cluster must have RBAC enabled, and your Tiller pod(s) therefore
require `cluster-admin` access.

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

## A Recent kubectl

As with Kubernetes itself, interaction with the service catalog system is
achieved through the `kubectl` command line interface. Chances are high that
you already have this installed, however, the service catalog *requires*
`kubectl` version 1.7 or newer.

To proceed, we must:

- Download and install `kubectl` version 1.7 or newer.
- Configure `kubectl` to communicate with the service catalog's API server.

To install `kubectl` follow the [standard instructions](https://kubernetes.io/docs/tasks/kubectl/install/).

For example, on Mac OS:

```console
curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/darwin/amd64/kubectl
chmod +x ./kubectl
```

We'll assume that all `kubectl` commands are using this newly-installed
executable.

# Step 2 - Install Service Catalog

Use Helm to install the Service Catalog. From the root of this repository:

```console
helm install svc-cat/catalog \
    --name catalog --namespace catalog
```
