# Overview

A cluster consists of nodes, each node with some allocatable resources (CPU, Memory, GPU, etc.).
Each pod consumes part of the allocatable resources of some cluster node.
As each pod runs exactly on one node resource space is fragmented.
Computation of overall remaining allocatable (or free) resources gives a rough estimation.
Pod based estimation of remaining allocatable resources is more precise.
It estimates a number of instances of a given pod that are still schedulable in the cluster given current free resources.

Possible applications:

* Detection of remaining schedulable resources per namespace or cluster
* Load balacing of pods among federated clusters (e.g. federated cluster can have a lot of overall free resource for 40 pods but only 20 pods can be scheduled)
* Monitoring of consumption of remaining schedulable resources

# Framework

The goal is to provide a framework that estimates a number of instances of a specified pod that would be scheduled in a cluster.
The framework consists of a scheduler (created by the scheduler factory), loobpack REST client that intercepts requests
and local caches over which the estimation operates.
Before each estimation is run, the framework captures the current state of the cluster.
Once captured, the framework translates each REST request into corresponding local cache operation.
Thus, the running cluster is free of any modification that would be caused by scheduling a pod into a node.
Estimation is then collection and provided via available channels.

As the scheduling itself is product of more than just the scheduler component,
admission controller configuration is available as well.

## Configuration

The framework configuration can be divided into three categories:
* analysis configuration
* scheduler configuration
* admissions configuration

The analysis configuration covers:
* location of kubeconfig file and address of running Apiserver (``kubeconfig`` and ``apiserver-config`` options)
* pod specification to be scheduled (``podspec`` option)
* number of instances of a specified pod that cause analysis to be stopped prematurely when reached (``maxLimit`` option)
* period of time between two consecutive runs of analysis (``period`` option)
* mode of resource space exploration (``resource-space-mode`` option)

The scheduler configuration covers:
* location of scheduler configuration file (``default-config`` option)

The admissions configuration covers:
* location of Apiserver admission controller configuration (``apiserver-config`` option)

When the ``period`` is specified, the framework is run in a watch mode and the continuous stream of estimations is provided via REST API.
Once the scheduler, resp. admission configuration is set, it can not be changed during the framework execution.

## Scheduler

The framework expects a scheduler that is created by the default scheduler factory.
By default, the framework expects scheduler configuration under ``config/default-scheduler.yaml`` file.
The configuration file contains corresponding Scheduler options. E.g.:

```yaml
port: 10251
address: 0.0.0.0
algorithmprovider: DefaultProvider
policyconfigfile: ""
enableprofiling: false
contenttype: application/vnd.kubernetes.protobuf
kubeapiqps: 50
kubeapiburst: 100
schedulername: default-scheduler
hardpodaffinitysymmetricweight: 1
failuredomains: kubernetes.io/hostname,failure-domain.beta.kubernetes.io/zone,failure-domain.beta.kubernetes.io/region
leaderelection:
  leaderelect: true
  leaseduration:
    duration: 15s
  renewdeadline:
    duration: 10s
  retryperiod:
    duration: 2s
```

To achieve the same behaviour as the scheduler running in a cluster,
it is recommended to use the same scheduler configuration (including a list of enabled predicates and priority functions).

## Admissions

To enable admissions ``--apiserver-config`` option needs to be set to a location of a configuration file.
The configuration file contains a subset of corresponding Apiserver options. E.g.:

```yaml
$ cat config/apiserver.yml
authorizationMode: "AlwaysAllow"
authorizationPolicyFile: ""
authorizationWebhookConfigFile: ""
authorizationWebhookCacheAuthorizedTtl: "5m0s"
authorizationWebhookCacheUnauthorizedTtl: "30s"
authorizationRbacSuperUser: ""
admissionControl: "NamespaceLifecycle,LimitRanger,SecurityContextDeny,ServiceAccount,ResourceQuota,AlwaysPullImages"
```

The analysis explores the entire resource space by default.
Since the ``ResourceQuota`` admission is namespace specific,
it is disabled by default even if it is specified in the configuration file.
To enable the admission the ``--resource-space-mode`` option needs to be set to ``ResourceSpacePartial``.

# Cluster capacity analysis

The pod-based analysis aims at estimation of a number of instances of specified pod which would be scheduled in a cluster.
The following assumptions hold:
* the pod is running instantly right after it is scheduled
* no interaction with Kubelet (i.e. no container running, no image pulling, no volumes handling)
* no interaction with Apiserver (entire interaction is carried over local caches)
* no interaction with controllers

The framework can be run either as a binary from a CLI or as an application in a pod.
Running cluster is assumed.

## Greedy analysis

By default, the entire resource space is explored.
The number of instances of a pod is limited only by available allocatable resources.
Although only a subset of the pod specification is actually used,
it is still recommended to provide valid data (including existing image name).
In future, the subset of the specification in use can grow based on available predicates and priority functions.

**Example**:

```sh
$ ./cluster-capacity --kubeconfig <path to kubeconfig> --master <API server address> --podspec=examples/pod.yaml --verbose
Pod requirements:
    - cpu: 150m
    - memory: 100Mi

The cluster can schedule 52 instance(s) of the pod.
Termination reason: FailedScheduling: pod (small-pod-52) failed to fit in any node
fit failure on node (kube-node-1): Insufficient cpu
fit failure on node (kube-node-4): Insufficient cpu
fit failure on node (kube-node-2): Insufficient cpu
fit failure on node (kube-node-3): Insufficient cpu


Pod distribution among nodes:
    - kube-node-1: 13 instance(s)
    - kube-node-4: 13 instance(s)
    - kube-node-2: 13 instance(s)
    - kube-node-3: 13 instance(s)
```

## Namespace scoped analysis

Each pod is a member of exactly one namespace.
A namespace can house additional objects that can further limit schedulability of pods.
Such as:
* limit ranges
* resource quota
* node selector (``openshift.io/node-selector`` annotation)

Limit ranges can force a maximal amount of resources per pod or per container.
Resource quota can limit a maximal amount of resource consumed by all pods in a namespace.
Node selector can limit a set of nodes on which a pod can be scheduled. 

Some of the limitations are completely pod based (e.g. node selector, limit ranges).
Some are enforced by scheduler predicates or by admissions (e.g. resource quota).
Thus, to take the pod based limitations into account,
the pod specification needs to be extended accordingly.
E.g. set ``spec.nodeSelector`` or ``spec.containers[i].resources``.

The framework provides ``genpod`` binary which generates the extended pod specification
based on the current limit ranges and node selector of the specified namespace.
E.g. to generate the specification for namespace NAMESPACE, you can run:

```sh
$ genpod --kubeconfig <path to kubeconfig> --master <API server address> --namespace NAMESPACE
```

With the limitations set up, the framework can provide the following estimation:

```sh
$ ./cluster-capacity --kubeconfig <path to kubeconfig> --master <API server address > --podspec=genpod.yaml --apiserver-config config/apiserver.yaml --verbose --resource-space-mode ResourceSpacePartial
Pod requirements:
    - cpu: 150m
    - memory: 100Mi

The cluster can schedule 4 instance(s) of the pod.
Termination reason: AdmissionControllerError: pods "small-pod-4" is forbidden: exceeded quota: compute-resources, requested: pods=1, used: pods=4, limited: pods=4

Pod distribution among nodes:
    - 127.0.0.1: 4 instance(s)
```

## Watch mode

In order to provide a continuous stream of estimations, the framework can be run in a watch mode (setting ``period`` flag).
Once run, the estimations are available at ``http://localhost:8081/capacity/status?watch=true`` address.
You can use ``curl`` to access the data:

```sh
$ curl http://localhost:8081/capacity/status?watch=true
[
  {
   "Timestamp": "2016-11-16T08:30:33.079973497Z",
   "PodRequirements": {
    "Cpu": "200m",
    "Memory": "100Mi"
   },
   "TotalInstances": 5,
   "NodesNumInstances": {
    "kube-node-2": 5
   },
   "FailReasons": {
    "FailType": "FailedScheduling",
    "FailMessage": "pod (cluster-capacity-stub-container-5) failed to fit in any node",
    "NodeFailures": {
     "kube-node-1": "MatchNodeSelector",
     "kube-node-2": "Insufficient cpu"
    }
   }
  },
  {
   "Timestamp": "2016-11-16T08:30:43.277040728Z",
   "PodRequirements": {
    "Cpu": "200m",
    "Memory": "100Mi"
   },
   "TotalInstances": 5,
   "NodesNumInstances": {
    "kube-node-2": 5
   },
   "FailReasons": {
    "FailType": "FailedScheduling",
    "FailMessage": "pod (cluster-capacity-stub-container-5) failed to fit in any node",
    "NodeFailures": {
     "kube-node-1": "MatchNodeSelector",
     "kube-node-2": "Insufficient cpu"
    }
   }
  }
 ]
...
```

Each estimation provides:
* time of estimation: ``Timestamp`` property
* pod specification: ``PodRequirements`` property
* number of scheduled instances: ``TotalInstances`` property
* distribution of pod instances among nodes: ``NodesNumInstances`` property
* reason why the estimation stopped: ``FailReasons`` property

The namespace node selector or limit ranges can change over time.
To update the pod specification during framework execution, you can send a POST request to ``http://localhost:8081/capacity/pod``.
E.g. with new specification in ``nspod.json`` you can run:

```sh
$ curl -T nspod.json -H "Content-Type: application/json" http://localhost:8081/capacity/pod
```

## Running the framework inside a pod

Running the framework as a binary from a CLI is suitable for use cases where a user has access to a running cluster (i.e. kubeconfig and certificates are available).
In other use cases it is more suitable to run the framework from within a pod (e.g. for users with purely web UI access).

The framework pod specification can consist of:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: cluster-capacity
  labels:
    name: cluster-capacity
spec:
  containers:
  - name: cluster-capacity
    image: docker.io/gofed/cluster-capacity:latest
    command:
    - "/bin/sh"
    - "-ec"
    - |
      echo "Generating pod"
      /bin/genpod --namespace=cluster-capacity >> /pod.yaml
      cat /pod.yaml
      echo "Running cluster capacity framework"
      /bin/cluster-capacity --period=1 --podspec=/pod.yaml --default-config /config/default-scheduler.yaml
    ports:
    - containerPort: 8081
```

The pod specification for the estimation is generated from a specified namespace.
Alternatively you can build your own base image over the default framework one and extend it with additional custom pod generators.

Once the framework pod is running, the pod specification for the estimation can be updated by sending POST request.
