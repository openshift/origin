# Installing Service Catalog on Clusters Running Kubernetes 1.6 (DEPRECATED)

This document contains instructions for installing the Service Catalog onto
Kubernetes clusters running version 1.6. Since Service Catalog
only officially supports versions 1.7 and later, these instructions are 
deprecated and may be removed at any time.

If you are running a Kubernetes cluster running version 1.7 or later, please 
see the [installation instructions for 1.7](./install-1.7.md).

# Step 1 - Prerequisites

## Starting Kubernetes with DNS

You *must* have a Kubernetes cluster with cluster DNS enabled. We can't list
instructions here for enabling cluster DNS for all Kubernetes cluster
installations, but here are a few notes:

* If you are using Google Container Engine or minikube, you likely have cluster
DNS enabled already.
* If you are using hack/local-up-cluster.sh, ensure the
`KUBE_ENABLE_CLUSTER_DNS` environment variable is set as follows:

```console
KUBE_ENABLE_CLUSTER_DNS=true hack/local-up-cluster.sh -O
```

## Helm

You *must* use [Helm](http://helm.sh/) v2.5.0 or newer in the installation steps
below.

If you already have an appropriate Helm version, execute `helm init` 
(if you haven't already) to install Tiller (the server-side component of Helm),
and you should be done with Helm setup.

If you don't already have an appropriate version, see the
[Helm installation instructions](https://github.com/kubernetes/helm/blob/master/docs/install.md).

If your kubernetes cluster has
[RBAC](https://kubernetes.io/docs/admin/authorization/rbac/) enabled, you must
ensure that the tiller pod has `cluster-admin` access. By default, `helm init`
installs the tiller pod into `kube-system` namespace, with tiller configured to
use the `default` service account.

```console
kubectl create clusterrolebinding tiller-cluster-admin --clusterrole=cluster-admin --serviceaccount=kube-system:default
```

`cluster-admin` access is required in order for helm to work correctly in
clusters with RBAC enabled.  If you used the `--tiller-namespace` or
`--service-account` flags when running `helm init`, the `--serviceaccount` flag
in the previous command needs to be adjusted to reference the appropriate
namespace and ServiceAccount name.

### Helm Charts

You need to download the
[charts/catalog](https://github.com/kubernetes-incubator/service-catalog/tree/master/charts/catalog)
directory to your local machine. Please refer to
[here](https://github.com/kubernetes-incubator/service-catalog/blob/master/docs/devguide.md#2-clone-fork-to-local-storage) 
for the guide.

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

We'll assume hereafter that all `kubectl` commands are using this
newly-installed executable.


# Step 2 - Installing the Service Catalog

The service catalog is packaged as a Helm chart located in the
[charts/catalog](../charts/catalog) directory in this repository, and supports a
wide variety of customizations which are detailed in that directory's
[README.md](../charts/catalog/README.md).

## The Service Catalog Data Store

We'll be interacting with a variety of resources in the following steps. The
service catalog API server needs to store all of these resources in a data
store. The data store implementation in the API server is pluggable, and we
currently support the following implementations:

1. Etcd 3
2. Third Party Resources (also, known as TPRs) - this is an _alpha_ feature 
right now. It has known issues and may be removed at any time.

The first implementation requires that the API server has access to an Etcd 3 cluster, and the
second only requires access to the Kubernetes API to store TPRs.

Even if you store data in TPRs, you should still access data via the service catalog API. It is 
possible to access data via the TPRs directly, but we don't recommend it.

## Install

To install the service catalog system with Etcd 3 as the backing data store:

```console
helm install charts/catalog --name catalog --namespace catalog
```

To install the service catalog system with TPRs as the backing data store:

```console
helm install charts/catalog --name catalog --namespace catalog --set apiserver.storage.type=tpr,apiserver.storage.tpr.globalNamespace=catalog
```

Regardless of which data store implementation you choose, the remainder of the steps in this
walkthrough will stay the same.

## API Server Authentication and Authorization

Authentication and authorization are disabled in the Helm chart by default. To enable them, 
set the `apiserver.auth.enabled` option on the Helm chart:

```console
helm install charts/catalog --name catalog --namespace catalog --set apiserver.auth.enabled=true
```

For more information about certificate setup, see the [documentation on
authentication and authorization](./auth.md).


## Do Overs

If you make a mistake somewhere along the way in this walk-through and want to 
start over, please run the following commands for clean-up.

```console
helm delete --purge catalog
kubectl delete ns catalog
```

## Step 3 - Configuring `kubectl` to Talk to the API Server

To configure `kubectl` to communicate with the service catalog API server, we'll have to
get the IP address that points to the `Service` that sits in front of the API server pod(s).
If you installed the catalog with one of the `helm install` commands above, then this service 
will be called `catalog-catalog-apiserver`, and be in the `catalog` namespace. 

### Notes on Getting the IP Address

How you get this IP address is highly dependent on your Kubernetes installation
method. Regardless of how you do it, do not use the Cluster IP of the 
`Service`. The `Service` is created as a `NodePort` in this walkthrough, you 
will need to use the address of one of the nodes in your cluster.

### Setting up a New `kubectl` Context

When you determine the IP address of this service, set its value into the `SVC_CAT_API_SERVER_IP`
environment variable and then run the following commands:

```console
kubectl config set-cluster service-catalog --server=https://${SVC_CAT_API_SERVER_IP}:30443 --insecure-skip-tls-verify=true
kubectl config set-context service-catalog --cluster=service-catalog
```

Note: Your cloud provider may require firewall rules to allow your traffic get in.
Please refer to the [Troubleshooting](./walkthrough-1.6.md#troubleshooting) 
section of the walkthrough document for details.
