# Service Catalog

Service Catalog is a Kubernetes Incubator project that provides a
Kubernetes-native workflow for integrating with 
[Open Service Brokers](https://www.openservicebrokerapi.org/)
to provision and bind to application dependencies like databases, object
storage, message-oriented middleware, and more.

For more information,
[visit the project on github](https://github.com/kubernetes-incubator/service-catalog).

## Prerequisites

- Kubernetes 1.7+ with Beta APIs enabled
- `charts/catalog` already exists in your local machine

## Installing the Chart

To install the chart with the release name `catalog`:

```bash
$ helm install . --name catalog --namespace catalog
```

## Uninstalling the Chart

To uninstall/delete the `catalog` deployment:

```bash
$ helm delete catalog
```

The command removes all the Kubernetes components associated with the chart and
deletes the release.

## Configuration

The following tables lists the configurable parameters of the Service Catalog
chart and their default values.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image` | apiserver image to use | `quay.io/kubernetes-service-catalog/service-catalog:v0.1.11` |
| `imagePullPolicy` | `imagePullPolicy` for the service catalog | `Always` |
| `apiserver.aggregator.priority` | Priority of the APIService. | `100` |
| `apiserver.aggregator.groupPriorityMinimum` | The minimum priority the group should have. | `10000` |
| `apiserver.aggregator.versionPriority` | The ordering of this API inside of the group | `20` |
| `apiserver.tls.requestHeaderCA` | Base64-encoded CA used to validate request-header authentication, when receiving delegated authentication from an aggregator. If not set, the service catalog API server will inherit this CA from the `extension-apiserver-authentication` ConfigMap if available. | `nil` |
| `apiserver.service.type` | Type of service; valid values are `LoadBalancer` and `NodePort` | `NodePort` |
| `apiserver.service.nodePort.securePort` | If service type is `NodePort`, specifies a port in allowable range (e.g. 30000 - 32767 on minikube); The TLS-enabled endpoint will be exposed here | `30443` |
| `apiserver.storage.type` | The storage backend to use; the only valid value is `etcd`, left for other storages support in future, e.g. `crd` | `etcd` |
| `apiserver.storage.etcd.useEmbedded` | If storage type is `etcd`: Whether to embed an etcd container in the apiserver pod; THIS IS INADEQUATE FOR PRODUCTION USE! | `true` |
| `apiserver.storage.etcd.servers` | If storage type is `etcd`: etcd URL(s); override this if NOT using embedded etcd | `http://localhost:2379` |
| `apiserver.storage.etcd.persistence.enabled` | Enable persistence using PVC | `false` |
| `apiserver.storage.etcd.persistence.storageClass` | PVC Storage Class | `nil` (uses alpha storage class annotation) |
| `apiserver.storage.etcd.persistence.accessMode` | PVC Access Mode | `ReadWriteOnce` |
| `apiserver.storage.etcd.persistence.size` | PVC Storage Request | `4Gi` |
| `apiserver.verbosity` | Log level; valid values are in the range 0 - 10 | `10` |
| `apiserver.auth.enabled` | Enable authentication and authorization | `true` |
| `apiserver.audit.activated` | If true, enables the use of audit features via this chart. | `false` |
| `apiserver.audit.logPath` | If specified, audit log goes to specified path. | `"/tmp/service-catalog-apiserver-audit.log"` |
| `apiserver.serviceAccount` | Service account. | `service-catalog-apiserver` |
| `apiserver.serveOpenAPISpec` | If true, makes the API server serve the OpenAPI schema | `false` |
| `controllerManager.verbosity` | Log level; valid values are in the range 0 - 10 | `10` |
| `controllerManager.resyncInterval` | How often the controller should resync informers; duration format (`20m`, `1h`, etc) | `5m` |
| `controllerManager.brokerRelistInterval` | How often the controller should relist the catalogs of ready brokers; duration format (`20m`, `1h`, etc) | `24h` |
| `controllerManager.brokerRelistIntervalActivated` | Whether or not the controller supports a --broker-relist-interval flag. If this is set to true, brokerRelistInterval will be used as the value for that flag. | `true` |
| `controllerManager.profiling.disabled` | Disable profiling via web interface host:port/debug/pprof/ | `false` |
| `controllerManager.profiling.contentionProfiling` | Enables lock contention profiling, if profiling is enabled | `false` |
| `controllerManager.leaderElection.activated` | Whether the controller has leader election enabled | `false` |
| `controllerManager.serviceAccount` | Service account | `service-catalog-controller-manager` |
| `controllerManager.apiserverSkipVerify` | Controls whether the API server's TLS verification should be skipped | `true` |
| `controllerManager.enablePrometheusScrape` | Whether the controller will expose metrics on /metrics | `false` |
| `useAggregator` | whether or not to set up the controller-manager to go through the main Kubernetes API server's API aggregator | `true` |
| `rbacEnable` | If true, create & use RBAC resources | `true` |
| `originatingIdentityEnabled` | Whether the OriginatingIdentity alpha feature should be enabled | `false` |
| `asyncBindingOperationsEnabled` | Whether or not alpha support for async binding operations is enabled | `false` |

Specify each parameter using the `--set key=value[,key=value]` argument to
`helm install`.

Alternatively, a YAML file that specifies the values for the parameters can be
provided while installing the chart. For example:

```bash
$ helm install . --name catalog --namespace catalog --values values.yaml
```
