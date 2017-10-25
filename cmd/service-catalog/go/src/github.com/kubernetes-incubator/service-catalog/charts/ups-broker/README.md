# User Provided Service Broker

User Provided Service Broker is an example
[Open Service Broker](https://www.openservicebrokerapi.org/)
for use demonstrating the Kubernetes
Service Catalog.

For more information,
[visit the Service Catalog project on github](https://github.com/kubernetes-incubator/service-catalog).

## Installing the Chart

To install the chart with the release name `ups-broker`:

```bash
$ helm install charts/ups-broker --name ups-broker --namespace ups-broker
```

## Uninstalling the Chart

To uninstall/delete the `ups-broker` deployment:

```bash
$ helm delete ups-broker
```

The command removes all the Kubernetes components associated with the chart and
deletes the release.

## Configuration

The following tables lists the configurable parameters of the User Provided
Service Broker

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image` | Image to use | `quay.io/kubernetes-service-catalog/user-broker:v0.1.0` |
| `imagePullPolicy` | `imagePullPolicy` for the ups-broker | `Always` |

Specify each parameter using the `--set key=value[,key=value]` argument to
`helm install`.

Alternatively, a YAML file that specifies the values for the parameters can be
provided while installing the chart. For example:

```bash
$ helm install charts/ups-broker --name ups-broker --namespace ups-broker \
  --values values.yaml
```
