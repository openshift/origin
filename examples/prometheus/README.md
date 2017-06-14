# Prometheus Ops Metrics Example

This template creates a Prometheus instance preconfigured to gather OpenShift and Kubernetes platform and node metrics and report them to admins. It is protected by an OAuth proxy that only allows access for users who have view access to the `kube-system` namespace.

To deploy, run:

```
$ oc new-app -f prometheus.yaml
```

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

