# Prometheus Ops Metrics Example

This template creates a Prometheus instance preconfigured to gather OpenShift and Kubernetes platform and node metrics and report them to admins. It is protected by an OAuth proxy that only allows access for users who have view access to the `kube-system` namespace.

To deploy, run:

```
$ oc new-app -f prometheus.yaml
```

You may customize where the images (built from `openshift/prometheus` and `openshift/oauth-proxy`) are pulled from via template parameters.

The optional `node-exporter` component may be installed as a daemon set to gather host level metrics. It requires additional
privileges to view the host and should only be run in administrator controlled namespaces.

To deploy, run:

```
$ oc create -f node-exporter.yaml -n kube-system
$ oc adm policy add-scc-to-user -z prometheus-node-exporter -n kube-system hostaccess
```

## Useful metrics queries

### Related to how much data is being gathered by Prometheus

> sort_desc( count by (__name__)({__name__=~".+"}))

Number of metrics series per metric (a series is a unique combination of labels for a given metric).

> scrape_samples_scraped

Number of samples (individual metric values) exposed by each endpoint at the time it was scraped.

### System and container CPU

> sum(machine_cpu_cores)

Total number of cores in the cluster.

> sum(sort_desc(rate(container_cpu_usage_seconds_total{id="/"}[5m])))

Total number of consumed cores.

> sort_desc(sum by (kubernetes_io_hostname,type) (rate(container_cpu_usage_seconds_total{id="/"}[5m])))

CPU consumed per node in the cluster.

> sort_desc(sum by (cpu,id,pod_name,container_name) (rate(container_cpu_usage_seconds_total{type="infra"}[5m])))

CPU consumption per system service or container on the infrastructure nodes.

> sort_desc(sum by (namespace) (rate(container_cpu_usage_seconds_total[5m])))

CPU consumed per namespace on the cluster.

> drop_common_labels(sort_desc(sum without (cpu) (rate(container_cpu_usage_seconds_total{container_name="prometheus"}[5m]))))

CPU per instance of Prometheus container.

> sum(rate(container_cpu_usage_seconds_total{id="/"}[3m])) / sum(machine_cpu_cores)

Percentage of total cluster CPU in use

> sum(container_memory_rss) / sum(machine_memory_bytes)

Percentage of total cluster memory in use

> sum by (kubernetes_io_hostname) (rate(container_cpu_usage_seconds_total{type="master",id=~"/system.slice/(docker\|etcd).service"}[10m]))

Aggregate CPU usage of several systemd units

### Changes in your cluster

> sum(changes(container_start_time_seconds[10m]))

The number of containers that start or restart over the last ten minutes.


### API related queries

> sort_desc(drop_common_labels(sum without (instance,type,code) (rate(apiserver_request_count{verb=~"POST|PUT|DELETE|PATCH"}[5m]))))

Number of mutating API requests being made to the control plane.

> sort_desc(drop_common_labels(sum without (instance,type,code) (rate(apiserver_request_count{verb=~"GET|LIST|WATCH"}[5m]))))

Number of non-mutating API requests being made to the control plane.

### Network Usage

> topk(10, (sum by (pod_name) (rate(container_network_receive_bytes_total[5m]))))

Top 10 pods doing the most receive network traffic

### etcd related queries

> etcd_disk_wal_fsync_duration_seconds_count{type="master"}

etcd "write-ahead-log" latency in milliseconds.  If this goes over 100ms, the cluster might destabilize.  Over 1000ms and things definitely start falling apart.

### Kubelet / docker related queries

> kubelet_docker_operations_latency_microseconds{type="compute",quantile="0.9"}

90th percentile latency for docker operations (in microseconds).  This number will include image pulls, so often will be hundreds of seconds.

> kubelet_docker_operations_timeout

Returns a running count (not a rate) of docker operations that have timed out since the kubelet was started.

> kubelet_docker_operations_errors

Returns a running count (not a rate) of docker operations that have failed since the kubelet was started.

> kubelet_pleg_relist_latency_microseconds

Returns PLEG (pod lifecycle event generator) latency metrics.  This represents the latency experienced by calls from the kubelet to the container runtime (i.e. docker or CRI-O).  High PLEG latency is often related to disk I/O performance on the docker storage partition.

### OpenShift build related queries

> count(openshift_build_active_time_seconds{phase="running"} < time() - 600)

Returns the number of builds that have been running for more than 10 minutes (600 seconds).

> count(openshift_build_active_time_seconds{phase="pending"} < time() - 600)

Returns the number of build that have been waiting at least 10 minutes (600 seconds) to start.

> sum(openshift_build_total{phase="failed"})

Returns the number of failed builds, regardless of the failure reason.

> openshift_build_total{phase="failed",reason="fetchsourcefailed"}

Returns the number of failed builds because of problems retrieving source from the associated Git repository.

> sum(openshift_build_total{phase="complete"})

Returns the number of successfully completed builds.

> openshift_build_total{phase="failed"} offset 5m

Returns the failed builds totals, per failure reason, from 5 minutes ago.