OpenShift Reserved Ports
========================

OpenShift reserves all ports on hosts that are reserved by [upstream Kubernetes](https://github.com/kubernetes/kubernetes/blob/master/pkg/master/ports/ports.go) as well as the following host ports:

| 2379,2380 | masters | [etcd](https://github.com/openshift/installer) | etcd API |
| 6443 | masters |[kube-apiserver](https://github.com/openshift/cluster-kube-apiserver-operator) | Kubernetes API |
| 9099 | masters | [Cluster Version Operator](https://github.com/openshift/cluster-version-operator) | Metrics |
| 9100, 9101 | nodes | [Node Exporter](https://github.com/openshift/node_exporter) | Metrics |
| 10256 | nodes | [openshift-sdn](https://github.com/openshift/origin) | Metrics |

By default, within an OpenShift cluster the port range 9000-9999 is accessible from all nodes in the cluster
for use by metrics scraping and other host level services. That is managed by the cloud security groups.