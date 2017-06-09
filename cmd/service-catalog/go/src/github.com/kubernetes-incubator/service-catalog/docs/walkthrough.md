# Service Catalog Demonstration Walkthrough

This document outlines the basic features of the service catalog by walking
through a short demo.

## Step 0 - Prerequisites

### Starting Kubernetes with DNS

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

### Getting Helm and installing Tiller

You *must* use [Helm](http://helm.sh/) v2 or newer in the installation steps
below.

If you already have Helm v2 or newer, execute `helm init` (if you haven't
already) to install Tiller (the server-side component of Helm), and you should
be done with Helm setup.

If you don't already have Helm v2, see the
[installation instructions](https://github.com/kubernetes/helm/blob/master/docs/install.md).

## Step 1 - Installing the Service Catalog

The service catalog is packaged as a Helm chart located in the
[charts/catalog](../charts/catalog) directory in this repository, and supports a
wide variety of customizations which are detailed in that directory's
[README.md](https://github.com/kubernetes-incubator/service-catalog/blob/master/charts/catalog/README.md).

### The Service Catalog Database

We'll be interacting with a variety of resources in the following steps. The service catalog API
server needs to store all of these resources in a database. The database implementation in the API
server is pluggable, and we currently support the following implementations:

1. Etcd 3
2. Third Party Resources (also, known as TPRs) - this is an _alpha_ feature right now. It has known issues

The first implementation requires that the API server has access to an Etcd 3 cluster, and the
second only requires access to the Kubernetes API to store TPRs.

Even if you store data in TPRs, you should still access data via the service catalog API. It is 
possible to access data via the TPRs directly, but we don't recommend it.

### Install

To install the service catalog system with Etcd 3 as the backing database:

```console
helm install charts/catalog --name catalog --namespace catalog
```

To install the service catalog system with TPRs as the backing database:

```console
helm install charts/catalog --name catalog --namespace catalog --set apiserver.storage.type=tpr,apiserver.storage.tpr.globalNamespace=catalog
```

Regardless of which database implementation you choose, the remainder of the steps in this
walkthrough will stay the same.

### API Server Authentication and Authorization

Authentication and authorization are disabled in the Helm chart by default. To enable them, 
set the `apiserver.auth.enabled` option on the Helm chart:

```console
helm install charts/catalog --name catalog --namespace catalog --set apiserver.auth.enabled=true
```

For more information about certificate setup, see the [documentation on
authentication and authorization](./auth.md).


### Do Overs

If you make a mistake somewhere along the way in this walk-through and want to start over,
check out the "Cleaning Up" section below. Follow those instructions before you start over.

## Step 2 - Understanding Service Catalog Components

The service catalog API has five main concepts:

- Broker Server: A server that acts as a service broker and conforms to the 
[Open Service Broker API](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md)
specification. This software could be hosted within your own Kubernetes cluster
or elsewhere.

The remaining four concepts all map directly to new Kubernetes resource types
that are provided by the service catalog API.

- `Broker`: An in-cluster representation of a broker server. A resource of this
type encapsulates connection details for that broker server. These are created
and managed by cluster operators who wish to use that broker server to make new
types of managed services available within their cluster.
- `ServiceClass`: A *type* of managed service offered by a particular broker.
Each time a new `Broker` resource is added to the cluster, the service catalog
controller connects to the corresponding broker server to obtain a list of
service offerings. A new `ServiceClass` resource will automatically be created
for each.
- `Instance`: A provisioned instance of a `ServiceClass`. These are created
by cluster users who wish to make a new concrete _instance_ of some _type_ of
managed service to make that available for use by one or more in-cluster
applications. When a new `Instance` resource is created, the service catalog
controller will connect to the appropriate broker server and instruct it to
provision the service instance.
- `Binding`: A "binding" to an `Instance`. These are created by cluster users
who wish for their applications to make use of a service `Instance`. Upon
creation, the service catalog controller will create a Kubernetes `Secret`
containing connection details and credentials for the service instance. Such
`Secret`s can be mounted into pods as usual.

These concepts and resources are the building blocks of the service catalog.

## Step 3 - Installing the UPS Broker

In order to effectively demonstrate the service catalog, we will require a
sample broker server. To proceed, we will deploy the [User Provided Service
broker (hereafter, "UPS")](https://github.com/kubernetes-incubator/service-catalog/tree/master/contrib/pkg/broker/user_provided/controller)
to our own Kubernetes cluster. Similar to the service catalog system itself,
this is easily installed using a provided Helm chart. The chart supports a
wide variety of customizations which are detailed in that directory's
[README.md](https://github.com/kubernetes-incubator/service-catalog/blob/master/charts/ups-broker/README.md).

**Note:** The UPS broker emulates user-provided services as they exist in
Cloud Foundry. Essentially, values provided during provisioning are merely
echoed during binding. (i.e. The values *are* the service.) This is a trivial
broker server but it's deliberately employed in this walkthrough to avoid
getting hung up on the distracting details of some other technology.

To install with defaults:

```console
helm install charts/ups-broker --name ups-broker --namespace ups-broker
```

## Step 4 - Installing the Latest `kubectl`

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

## Step 5 - Configuring `kubectl` to Talk to the API Server

To configure `kubectl` to communicate with the service catalog API server, we'll have to
get the IP address that points to the `Service` that sits in front of the API server pod(s).
If you installed the catalog with one of the `helm install` commands above, then this service 
will be called `catalog-catalog-apiserver`, and be in the `catalog` namespace. 

### Notes on Getting the IP Address

How you get this IP address is highly dependent on your Kubernetes installation method. Regardless
of how you do it, do not use the Cluster IP of the `Service`. The `Service` is created as a
`NodePort` in this walkthrough, so you'll likely need to use the IP address of the node or one of
the nodes in your cluster.

### Setting up a New `kubectl` Context

When you determine the IP address of this service, set its value into the `SVC_CAT_API_SERVER_IP`
environment variable and then run the following commands:

```console
kubectl config set-cluster service-catalog --server=http://$SVC_CAT_API_SERVER_IP:30080
kubectl config set-context service-catalog --cluster=service-catalog
```

## Step 6 - Creating a Broker Resource

Next, we'll register a broker server with the catalog by creating a new
[`Broker`](../contrib/examples/walkthrough/ups-broker.yaml) resource.

Because we haven't created any resources in the service-catalog API server yet,
`kubectl get` will return an empty list of resources.

```console
kubectl --context=service-catalog get brokers,serviceclasses,instances,bindings
No resources found
```

Create the new `Broker` resource with the following command:

```console
kubectl --context=service-catalog create -f contrib/examples/walkthrough/ups-broker.yaml
```

The output of that command should be the following:

```console
broker "ups-broker" created
```

When we create this `Broker` resource, the service catalog controller responds
by querying the broker server to see what services it offers and creates a
`ServiceClass` for each.

We can check the status of the broker using `kubectl get`:

```console
kubectl --context=service-catalog get brokers ups-broker -o yaml
```

We should see something like:

```yaml
apiVersion: servicecatalog.k8s.io/v1alpha1
kind: Broker
metadata:
  creationTimestamp: 2017-03-03T04:11:17Z
  finalizers:
  - kubernetes
  name: ups-broker
  resourceVersion: "6"
  selfLink: /apis/servicecatalog.k8s.io/v1alpha1/brokers/ups-broker
  uid: 72fa629b-ffc7-11e6-b111-0242ac110005
spec:
  url: http://ups-broker.ups-broker.svc.cluster.local:8000
status:
  conditions:
  - message: Successfully fetched catalog from broker
    reason: FetchedCatalog
    status: "True"
    type: Ready
```

Notice that the `status` field has been set to reflect that the broker server's
catalog of service offerings has been successfully added to our cluster's
service catalog.

## Step 7 - Viewing ServiceClasses

The controller created a `ServiceClass` for each service that the UPS broker
provides. We can view the `ServiceClass` resources available in the cluster by
executing:

```console
kubectl --context=service-catalog get serviceclasses
```

We should see something like:

```console
NAME                    KIND
user-provided-service   ServiceClass.v1alpha1.servicecatalog.k8s.io
```

As we can see, the UPS broker provides a type of service called
`user-provided-service`. Run the following command to see the details of this
offering:

```console
kubectl --context=service-catalog get serviceclasses user-provided-service -o yaml
```

We should see something like:

```yaml
apiVersion: servicecatalog.k8s.io/v1alpha1
kind: ServiceClass
metadata:
  creationTimestamp: 2017-03-03T04:11:17Z
  name: user-provided-service
  resourceVersion: "7"
  selfLink: /apis/servicecatalog.k8s.io/v1alpha1/serviceclassesuser-provided-service
  uid: 72fef5ce-ffc7-11e6-b111-0242ac110005
brokerName: ups-broker
externalID: 4F6E6CF6-FFDD-425F-A2C7-3C9258AD2468
bindable: false
planUpdatable: false
plans:
- name: default
  osbFree: true
  externalID: 86064792-7ea2-467b-af93-ac9694d96d52
```

## Step 8 - Provisioning a New Instance

Now that a `ServiceClass` named `user-provided-service` exists within our
cluster's service catalog, we can provision an instance of that. We do so by
creating a new [`Instance`](../contrib/examples/walkthrough/ups-instance.yaml)
resource.

Unlike `Broker` and `ServiceClass` resources, `Instance` resources must reside
within a Kubernetes namespace. To proceed, we'll first ensure that the namespace
`test-ns` exists:

```console
kubectl create namespace test-ns
```

We can then continue to create an `Instance`:

```console
kubectl --context=service-catalog create -f contrib/examples/walkthrough/ups-instance.yaml
```

That operation should output:

```console
instance "ups-instance" created
```

After the `Instance` is created, the service catalog controller will communicate
with the appropriate broker server to initiate provisioning. We can check the
status of this process like so:

```console
kubectl --context=service-catalog get instances -n test-ns ups-instance -o yaml
```

We should see something like:

```yaml
apiVersion: servicecatalog.k8s.io/v1alpha1
kind: Instance
metadata:
  creationTimestamp: 2017-03-03T04:26:08Z
  name: ups-instance
  namespace: test-ns
  resourceVersion: "9"
  selfLink: /apis/servicecatalog.k8s.io/v1alpha1/namespaces/test-ns/instances/ups-instance
  uid: 8654e626-ffc9-11e6-b111-0242ac110005
spec:
  externalID: 34c984e1-4626-4574-8a95-9e500d0d48d3
  planName: default
  serviceClassName: user-provided-service
status:
  conditions:
  - message: The instance was provisioned successfully
    reason: ProvisionedSuccessfully
    status: "True"
    type: Ready
```

## Step 9 - Binding to the Instance

Now that our `Instance` has been created, we can bind to it. To accomplish this,
we will create a [`Binding`](../contrib/examples/walkthrough/ups-binding.yaml)
resource.

```console
kubectl --context=service-catalog create -f contrib/examples/walkthrough/ups-binding.yaml
```

That command should output:

```console
binding "ups-binding" created
```

After the `Binding` resource is created, the service catalog controller will
communicate with the appropriate broker server to initiate binding. Generally,
this will cause the broker server to create and issue credentials that the
service catalog controller will insert into a Kubernetes `Secret`. We can check
the status of this process like so:

```console
kubectl --context=service-catalog get bindings -n test-ns ups-binding -o yaml
```

We should see something like:

```yaml
apiVersion: servicecatalog.k8s.io/v1alpha1
kind: Binding
metadata:
  creationTimestamp: 2017-03-07T01:44:36Z
  finalizers:
  - kubernetes
  name: ups-binding
  namespace: test-ns
  resourceVersion: "29"
  selfLink: /apis/servicecatalog.k8s.io/v1alpha1/namespaces/test-ns/bindings/ups-binding
  uid: 9eb2cdce-02d7-11e7-8edb-0242ac110005
spec:
  instanceRef:
    name: ups-instance
  externalID: b041db94-a5a0-41a2-87ae-1025ba760918
  secretName: my-secret
status:
  conditions:
  - message: Injected bind result
    reason: InjectedBindResult
    status: "True"
    type: Ready
```

Notice that the status has a `Ready` condition set.  This means our binding is
ready to use!  If we look at the `Secret`s in our `test-ns` namespace, we should
see a new one:

```console
kubectl get secrets -n test-ns
NAME                  TYPE                                  DATA      AGE
default-token-3k61z   kubernetes.io/service-account-token   3         29m
my-secret             Opaque                                2         1m
```

Notice that a new `Secret` named `my-secret` has been created.

## Step 10 - Unbinding from the Instance

Now, let's unbind from the instance.  To do this, we simply *delete* the
`Binding` resource that we previously created:

```console
kubectl --context=service-catalog delete -n test-ns bindings ups-binding
```

Checking the `Secret`s in the `test-ns` namespace, we should see that
`my-secret` has also been deleted:

```console
kubectl get secrets -n test-ns
NAME                  TYPE                                  DATA      AGE
default-token-3k61z   kubernetes.io/service-account-token   3         30m
```

## Step 11 - Deprovisioning the Instance

Now, we can deprovision the instance.  To do this, we simply *delete* the
`Instance` resource that we previously created:

```console
kubectl --context=service-catalog delete -n test-ns instances ups-instance
```

## Step 12 - Deleting the Broker

Next, we should remove the broker server, and the services it offers, from the catalog. We can do
so by simply deleting the broker:

```console
kubectl --context=service-catalog delete brokers ups-broker
```

We should then see that all the `ServiceClass` resources that came from that
broker have also been deleted:

```console
kubectl --context=service-catalog get serviceclasses
No resources found
```

## Step 13 - Final Cleanup

To clean up, delete all our helm deployments:

```console
helm delete --purge catalog ups-broker
```

Then, delete all the namespaces we created:

```console
kubectl delete ns test-ns catalog ups-broker
```
