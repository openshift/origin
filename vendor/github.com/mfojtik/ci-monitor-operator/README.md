## ci-monitor-operator

The purpose of this operator is to provide precise timeline for changes in cluster operators during the CI runs.
This operator will record all changes to `*.config.openshift.io` custom resources and store the deltas as GIT commits
to the local repository. It exposes a simple HTTP server that can be used to clone this repository.

Additionally, this operator runs a controller that reflects the condition status of every cluster operator to Prometheus
metrics. 

Example:

```
# HELP openshift_ci_monitor_operator_cluster_operator_status [ALPHA] A metric that tracks individual cluster operator status.
# TYPE openshift_ci_monitor_operator_cluster_operator_status gauge
openshift_ci_monitor_operator_cluster_operator_status{condition="Available",name="authentication",status="True"} 1.586055762e+09
openshift_ci_monitor_operator_cluster_operator_status{condition="Available",name="cloud-credential",status="True"} 1.586054345e+09
openshift_ci_monitor_operator_cluster_operator_status{condition="Available",name="cluster-autoscaler",status="True"} 1.586055046e+09
openshift_ci_monitor_operator_cluster_operator_status{condition="Available",name="config-operator",status="True"} 1.586054669e+09
openshift_ci_monitor_operator_cluster_operator_status{condition="Available",name="console",status="True"} 1.58605565e+09
```

To retrieve the metrics when running locally, you can use this command:

```shell script
oc get --loglevel=10 --insecure-skip-tls-verify --server=https://localhost:8443 --raw /metrics
```

### Deploying

Clone this repository and run the following command:

```bash
$ oc apply -f ./manifests
```

If you change something and built your own image, you have to subsitute the default image in deployment.

### Development

To run this operator locally, you will need a admin `kubeconfig` file. In next step, create the `ci-monitor-operator` namespace.
Then run `make` command which will output a binary you can run using:

```shell script
oc delete configmaps --all -n ci-monitor-operator # cleanup locks
REPOSITORY_PATH=/tmp/repository ./ci-monitor-operator operator --kubeconfig ~/kubeconfig --namespace=ci-monitor-operator
```

You will find the Git change log inside /tmp/repository.

