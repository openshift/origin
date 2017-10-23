# Service Catalog

Service Catalog is a Kubernetes Incubator project that provides a
Kubernetes-native workflow for integrating with 
[Open Service Brokers](https://www.openservicebrokerapi.org/)
to provision and bind to application dependencies like databases, object
storage, message-oriented middleware, and more.

For more information,
[visit the project on github](https://github.com/kubernetes-incubator/service-catalog).

## Prerequisites

- Kubernetes 1.6+ with Beta APIs enabled
- `charts/catalog` already exists in your local machine

## Installing the Chart

To install the chart with the release name `catalog`:

```bash
$ helm install charts/catalog --name catalog --namespace catalog
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
| `image` | apiserver image to use | `quay.io/kubernetes-service-catalog/service-catalog:v0.1.0-rc2` |
| `imagePullPolicy` | `imagePullPolicy` for the service catalog | `Always` |
| `apiserver.tls.cert` | Base64-encoded x509 certificate | A self-signed certificate |
| `apiserver.tls.key` | Base64-encoded private key | The private key for the certificate above |
| `apiserver.tls.ca` | Base64-encoded CA certificate used to sign the above certificate | |
| `apiserver.tls.requestHeaderCA` | Base64-encoded CA used to validate request-header authentication, when receiving delegated authentication from an aggregator | *none (will disable requestheader authentication)* |
| `apiserver.service.type` | Type of service; valid values are `LoadBalancer` and `NodePort` | `NodePort` |
| `apiserver.service.nodePort.securePort` | If service type is `NodePort`, specifies a port in allowable range (e.g. 30000 - 32767 on minikube); The TLS-enabled endpoint will be exposed here | `30443` |
| `apiserver.storage.type` | The storage backend to use; the only valid value is `etcd`, left for other storages support in future, e.g. `crd` | `etcd` |
| `apiserver.storage.etcd.useEmbedded` | If storage type is `etcd`: Whether to embed an etcd container in the apiserver pod; THIS IS INADEQUATE FOR PRODUCTION USE! | `true` |
| `apiserver.storage.etcd.servers` | If storage type is `etcd`: etcd URL(s); override this if NOT using embedded etcd | `http://localhost:2379` |
| `apiserver.verbosity` | Log level; valid values are in the range 0 - 10 | `10` |
| `apiserver.auth.enabled` | Enable authentication and authorization | `false` |
| `controllerManager.verbosity` | Log level; valid values are in the range 0 - 10 | `10` |
| `controllerManager.resyncInterval` | How often the controller should resync informers; duration format (`20m`, `1h`, etc) | `5m` |
| `controllerManager.brokerRelistInterval` | How often the controller should relist the catalogs of ready brokers; duration format (`20m`, `1h`, etc) | `24h` |
| `useAggregator` | whether or not to set up the controller-manager to go through the main Kubernetes API server's API aggregator (requires setting `apiserver.tls.ca` to work) | `false` |
| `rbacEnable` | If true, create & use RBAC resources | `true` |

Specify each parameter using the `--set key=value[,key=value]` argument to
`helm install`.

Alternatively, a YAML file that specifies the values for the parameters can be
provided while installing the chart. For example:

```bash
$ helm install charts/catalog --name catalog --namespace catalog \
  --values values.yaml
```
