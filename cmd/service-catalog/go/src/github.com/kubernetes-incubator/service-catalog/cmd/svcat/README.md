# Service Catalog CLI

This is a command-line interface (CLI) for interacting with
[Kubernetes Service Catalog](../../README.md)
resources. svcat is a domain-specific tool to make interacting with the Service Catalog easier.
While many of its commands have analogs to `kubectl`, our goal is to streamline and optimize
the operator experience, contributing useful code back upstream to Kubernetes where applicable.

svcat communicates with the Service Catalog API through the [aggregated API][agg-api] endpoint on a
Kubernetes cluster.

[agg-api]: https://kubernetes.io/docs/concepts/api-extension/apiserver-aggregation/

# Prerequisites
In order to use svcat, you will need:

* A Kubernetes cluster running v1.7+ or higher
* A broker compatible with the Open Service Broker API installed on the cluster, such as:
  * [User Provided Service Broker](../../charts/ups-broker)
  * [Open Service Broke for Azure](https://github.com/Azure/helm-charts/tree/master/open-service-broker-azure)
* The [Service Catalog](../../docs/install.md) installed on the cluster.

# Install
Follow the appropriate instructions for your shell to download svcat. The binary
can be used by itself, or as kubectl plugin.

## Bash
```
curl -sLO https://servicecatalogcli.blob.core.windows.net/cli/latest/$(uname -s)/$(uname -m)/svcat
chmod +x ./svcat
mv ./svcat /usr/local/bin/
svcat --version
```

## PowerShell

```
iwr 'https://servicecatalogcli.blob.core.windows.net/cli/latest/Windows/x86_64/svcat.exe' -UseBasicParsing -OutFile svcat.exe
mkdir -f ~\bin
$env:PATH += ";${pwd}\bin"
svcat --version
```

The snippet above adds a directory to your PATH for the current session only.
You will need to find a permanent location for it and add it to your PATH.

## Manual
1. Download the appropriate binary for your operating system:
    * macOS: https://servicecatalogcli.blob.core.windows.net/cli/latest/Darwin/x86_64/svcat
    * Windows: https://servicecatalogcli.blob.core.windows.net/cli/latest/Windows/x86_64/svcat.exe
    * Linux: https://servicecatalogcli.blob.core.windows.net/cli/latest/Linux/x86_64/svcat
1. Make the binary executable.
1. Move the binary to a directory on your PATH.

## Plugin
To use svcat as a plugin, run the following command after downloading:

```console
$ ./svcat install plugin
Plugin has been installed to ~/.kube/plugins/svcat. Run kubectl plugin svcat --help for help using the plugin.
```

When operating as a plugin, the commands are the same with the addition of the global
kubectl configuration flags. One exception is that boolean flags aren't supported
when running in plugin mode, so instead of using `--flag` you must specify a value `--flag=true`.


# Use

Run `svcat -h` to see the available commands.

Below are some common tasks made easy with svcat. The example output assumes that [User Provided Service Broker](../../charts/ups-broker) is installed on the cluster.

## Find brokers installed on the cluster

```console
$ svcat get brokers
     NAME                                 URL                              STATUS
+------------+-----------------------------------------------------------+--------+
  ups-broker   http://ups-broker-ups-broker.ups-broker.svc.cluster.local   Ready
```

## Trigger a sync of a broker's catalog

```console
$ svcat sync broker ups-broker
Successfully fetched catalog entries from the ups-broker broker
```

## List available service classes

```console
$ svcat get classes
          NAME                  DESCRIPTION                         UUID
+-----------------------+-------------------------+--------------------------------------+
  user-provided-service   A user provided service   4f6e6cf6-ffdd-425f-a2c7-3c9258ad2468
```

## View service plans associated with a class

```console
$ svcat describe class user-provided-service
  Name:          user-provided-service
  Description:   A user provided service
  UUID:          4f6e6cf6-ffdd-425f-a2c7-3c9258ad2468
  Status:        Active
  Tags:
  Broker:        ups-broker

Plans:
   NAME           DESCRIPTION
+---------+-------------------------+
  default   Sample plan description
  premium   Premium plan
```

## Provision a service

```console
$ svcat provision -n test-ns ups-instance --class user-provided-service --plan default
  Name:        ups-instance
  Namespace:   test-ns
  Status:
  Class:       user-provided-service
  Plan:        default
```

Additional parameters and secrets can be provided using the `--param` and `--secret` flags:

```
--param p1=foo --param p2=bar --secret creds[db]
```

## View all instances of a service plan on the cluster

```console
$ svcat describe plan premium
  Name:          premium
  Description:   Premium plan
  UUID:          cc0d7529-18e8-416d-8946-6f7456acd589
  Status:        Active
  Free:          false
  Class:         user-provided-service
  
Instances:
      NAME       NAMESPACE   STATUS
+--------------+-----------+--------+
  ups-instance   test-ns     Ready
```

## List all service instances in a namespace

```console
$ svcat get instances -n test-ns
    NAME       NAMESPACE           CLASS            PLAN     STATUS
+--------------+-----------+-----------------------+---------+--------+
ups-instance   test-ns     user-provided-service   default   Ready
```

## Bind an instance

```console
$ svcat bind -n test-ns ups-instance --name ups-binding
  Name:        ups-binding
  Namespace:   test-ns
  Status:
  Instance:    ups-instance
```

When omitted, the names of the binding and secret are defaulted to the name of the instance.

```console
$ svcat bind ups
  Name:        ups
  Namespace:   default
  Status:
  Instance:    ups
```

## View the details of a service instance

```console
$ svcat describe instance -n test-ns ups-instance
  Name:        ups-instance
  Namespace:   test-ns
  Status:      Ready - The instance was provisioned successfully @ 2018-01-11 14:59:47 -0600 CST
  Class:       user-provided-service
  Plan:        default

Bindings:
     NAME       STATUS
+-------------+--------+
  ups-binding   Ready
```

## Unbind all applications from an instance
```
svcat unbind -n test-ns ups-instance
```

## Unbind a single application from an instance
```
svcat unbind -n test-ns --name ups-binding
```

## Delete a service instance
Deprovisioning is the process of preparing an instance to be removed, and then deleting it.

```
svcat deprovision ups-instance
```
