# Cluster capacity analysis framework

Implementation of [cluster capacity analysis](https://github.com/kubernetes-incubator/cluster-capacity/blob/master/doc/cluster-capacity.md).

## Intro

As new pods get scheduled on nodes in a cluster, more resources get consumed.
Monitoring available resources in the cluster is very important
as operators can increase the current resources in time before all of them get exhausted.
Or, carry different steps that lead to increase of available resources.

Cluster capacity consists of capacities of individual cluster nodes.
Capacity covers CPU, memory, disk space and other resources.

Overall remaining allocatable capacity is a rough estimation since it does not assume all resources being distributed among nodes.
Goal is to analyze remaining allocatable resources and estimate available capacity that is still consumable
in terms of a number of instances of a pod with given requirements that can be scheduled in a cluster.

## Build and Run

Build the framework:

```sh
$ make build
```

and run the analysis:

```sh
$ ./cluster-capacity --kubeconfig <path to kubeconfig> --podspec=examples/pod.yaml
```

For more information about available options run:
```
$ ./cluster-capacity --help
```

## Demonstration

Assuming a cluster is running with 4 nodes and 1 master with each node with 2 CPUs and 4GB of memory.
With pod resource requirements to be `150m` of CPU and ``100Mi`` of Memory.

```sh
$ ./cluster-capacity --kubeconfig <path to kubeconfig> --podspec=pod.yaml --verbose
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

To decrease available resources in the cluster you can use provided RC (`examples/rc.yml`):

```sh
$ kubectl create -f examples/rc.yml
```

E.g. to change a number of replicas to `6`, you can run:

```sh
$ kubectl patch -f examples/rc.yml -p '{"spec":{"replicas":6}}'
```

Once the number of running pods in the cluster grows and the analysis is run again,
the number of schedulable pods decreases as well:

```sh
$ ./cluster-capacity --kubeconfig <path to kubeconfig> --podspec=pod.yaml --verbose
Pod requirements:
	- cpu: 150m
	- memory: 100Mi

The cluster can schedule 46 instance(s) of the pod.
Termination reason: FailedScheduling: pod (small-pod-46) failed to fit in any node
fit failure on node (kube-node-1): Insufficient cpu
fit failure on node (kube-node-4): Insufficient cpu
fit failure on node (kube-node-2): Insufficient cpu
fit failure on node (kube-node-3): Insufficient cpu


Pod distribution among nodes:
	- kube-node-1: 11 instance(s)
	- kube-node-4: 12 instance(s)
	- kube-node-2: 11 instance(s)
	- kube-node-3: 12 instance(s)
```

## Output format
`cluster capacity` command has a flag `--output (-o)` to format its output as json or yaml.

```sh
$ ./cluster-capacity --kubeconfig <path to kubeconfig> --podspec=pod.yaml -o json
$ ./cluster-capacity --kubeconfig <path to kubeconfig> --podspec=pod.yaml -o yaml
```

The json or yaml output is not versioned and is not guaranteed to be stable across various releases.


## Pod spec generator: genpod

`genpod` is an internal tool to cluster capacity, and could be used to create sample pod spec.
In general, users are recommended to provide their own pod spec file as part of analysis

As pods are part of a namespace with resource limits and additional constraints (e.g. node selector forced by namespace annotation),
it is natural to analyse how many instances of a pod with maximal resource requirements can be scheduled.
In order to generate the pod spec, you can run:

```sh
$ genpod --kubeconfig <path to kubeconfig>  --namespace <namespace>
```

Assuming at least one resource limits object is available with at least one maximum resource type per pod.
If multiple resource limits objects per namespace are available, minimum of all maximum resources per type is taken.
If a namespace is annotated with `openshift.io/node-selector`, the selector is set as pod's node selector.

**Example**:

Assuming `cluster-capacity` namespace with `openshift.io/node-selector: "region=hpc,load=high"` annotation
and resource limits are created (see `examples/namespace.yml` and `examples/limits.yml`)

```sh
$ kubectl describe limits hpclimits --namespace cluster-capacity
Name:           hpclimits
Namespace:      cluster-capacity
Type            Resource        Min     Max     Default Request Default Limit   Max Limit/Request Ratio
----            --------        ---     ---     --------------- -------------   -----------------------
Pod             cpu             10m     200m    -               -               -
Pod             memory          6Mi     100Mi   -               -               -
Container       memory          6Mi     20Mi    6Mi             6Mi             -
Container       cpu             10m     50m     10m             10m             -

```

```sh
$ genpod --kubeconfig <path to kubeconfig>  --namespace cluster-capacity
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: null
  name: cluster-capacity-stub-container
  namespace: cluster-capacity
spec:
  containers:
  - image: gcr.io/google_containers/pause:2.0
    imagePullPolicy: Always
    name: cluster-capacity-stub-container
    resources:
      limits:
        cpu: 200m
        memory: 100Mi
      requests:
        cpu: 200m
        memory: 100Mi
  dnsPolicy: Default
  nodeSelector:
    load: high
    region: hpc
  restartPolicy: OnFailure
status: {}
```

## Roadmap

Underway:

* analysis covering scheduler and admission controller
* generic framework for any scheduler created by the default scheduler factory
* continuous stream of estimations

Would like to get soon:

* include multiple schedulers
* accept a list (sequence) of pods
* extend analysis with volume handling
* define common interface each scheduler need to implement if embedded in the framework

Other possibilities:

* incorporate re-scheduler
* incorporate preemptive scheduling
* include more of Kubelet's behaviour (e.g. recognize memory pressure, secrets/configmap existence test)
