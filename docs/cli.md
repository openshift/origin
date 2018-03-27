# Service Catalog CLI

This is a command-line interface (CLI) for interacting with Service Catalog
resources. svcat is a domain-specific tool to make interacting with the Service Catalog easier.
While many of its commands have analogs to `kubectl`, our goal is to streamline and optimize
the operator experience.

svcat communicates with the Service Catalog API through the [aggregated API][agg-api] endpoint on a
Kubernetes cluster.

[agg-api]: https://kubernetes.io/docs/concepts/api-extension/apiserver-aggregation/

This document assumes that you've installed Service Catalog and the Service Catalog CLI
onto your cluster. If you haven't, please see [install.md](install.md).

## Plugin
To use svcat as a kubectl plugin, run the following command after downloading:

```console
$ svcat install plugin
Plugin has been installed to ~/.kube/plugins/svcat. Run kubectl plugin svcat --help for help using the plugin.
```

When operating as a plugin, the commands are the same with the addition of the global
kubectl configuration flags. One exception is that boolean flags aren't supported
when running in plugin mode, so instead of using `--flag` you must specify a value `--flag=true`.


# Use

Run `svcat --help` to see the available commands.

Below are some common tasks made easy with svcat. The example output assumes that the 
[User Provided Service Broker](../charts/ups-broker) is installed on the cluster.

* [Find brokers installed on the cluster](#find-brokers-installed-on-the-cluster)
* [Trigger a sync of a broker's catalog](#trigger-a-sync-of-a-brokers-catalog)
* [List available service classes](#list-available-service-classes)
* [View service plans associated with a class](#view-service-plans-associated-with-a-class)
* [Provision a service](#provision-a-service)
* [View all instances of a service plan on the cluster](#view-all-instances-of-a-service-plan-on-the-cluster)
* [List all service instances in a namespace](#list-all-service-instances-in-a-namespace)
* [Bind an instance](#bind-an-instance)
* [View the details of a service instance](#view-the-details-of-a-service-instance)
* [Unbind all applications from an instance](#remove-all-bindings-from-an-instance)
* [Unbind a single application from an instance](#remove-a-single-binding-from-an-instance)
* [Delete a service instance](#remove-a-single-binding-from-an-instance)

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
                NAME                        DESCRIPTION                         UUID
+-----------------------------------+-------------------------+--------------------------------------+
  user-provided-service               A user provided service   4f6e6cf6-ffdd-425f-a2c7-3c9258ad2468
  user-provided-service-single-plan   A user provided service   5f6e6cf6-ffdd-425f-a2c7-3c9258ad2468
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

You can also provide provision parameters in the form of a JSON string using the `--params-json` flag:

```
svcat provision secure-instance --class mysqldb --plan secureDB --params-json '{
    "encrypt" : true,
    "firewallRules" : [
        {
            "name": "AllowSome",
            "startIPAddress": "75.70.113.50",
            "endIPAddress" : "75.70.113.131"
        },
        {
            "name": "AllowMore",
            "startIPAddress": "13.54.0.0",
            "endIPAddress" : "13.56.0.0"
        }
    ]
}
'
```

Note: You may not combine the `--params-json` flag with individual `--param` flags.

## View all instances of a service plan on the cluster
When there is more than one plan with the same name, the class can be provided either as a prefix to the plan name,
`CLASS/PLAN`, or specified with the class flag, `--class CLASS`.

```console
$ svcat describe plan user-provided-service/default
    Name:          default
    Description:   Sample plan description
    UUID:          86064792-7ea2-467b-af93-ac9694d96d52
    Status:        Active
    Free:          true
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
    Status:      Ready - The instance was provisioned successfully @ 2018-03-02 16:24:55 +0000 UTC
    Class:       user-provided-service
    Plan:        default
  
  Bindings:
       NAME       STATUS
  +-------------+--------+
    ups-binding   Ready
```

## Remove all bindings from an instance

```console
$ svcat unbind -n test-ns ups-instance
deleted ups-binding
```

## Remove a single binding from an instance

```console
$ svcat unbind -n test-ns --name ups-binding
deleted ups-binding
```

## Delete a service instance

Deprovisioning is the process of preparing an instance to be removed, and then deleting it.
You must unbind delete all bindings before deprovisioning an instance.

```console
$ svcat deprovision ups-instance
deleted ups-instance
```
