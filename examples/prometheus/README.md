# Prometheus Ops Metrics Example

This template creates a Prometheus instance preconfigured to gather OpenShift and Kubernetes platform and node metrics and report them to admins. It is protected by an OAuth proxy that only allows access for users who have view access to the `kube-system` namespace.

To deploy, run:

```
$ oc new-app -f https://raw.githubusercontent.com/openshift/origin/master/examples/prometheus/prometheus.yaml
```

This template sets up an oauth proxy for authentication.  The oauth-proxy relies on the same authentication system configured for your OpenShift cluster.

See [this documentation](https://docs.openshift.org/latest/install_config/configuring_authentication.html) for authentication options for OpenShift.

One option documented above is htpasswd.  Create an htpasswd user called (for example) ```prometheus```.

The ```prometheus``` user must have cluster-reader access.  Grant the user cluster-reader access:
```
$ oc adm policy add-cluster-role-to-user cluster-reader prometheus
```

Query the system for the route created by the template:
```
# oc get routes -n kube-system | grep prom
prometheus   prometheus-kube-system.NNNN-NNN.NN.rhcloud.com             prometheus   <all>     reencrypt     None
```

Open the route URL in your browser, and you should be able to authenticate using the ```prometheus``` user and the corresponding password.

You may customize where the images (built from `openshift/prometheus` and `openshift/oauth-proxy`) are pulled from via template parameters.

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

