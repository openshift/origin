# Cluster Mirror

This document describes how to use the Cluster Mirror to create a Cluster
Loader configuration of a local OpenShift cluster. Specifically the templates
and pods that are contained in the various cluster namespaces.

Running Cluster Mirror
----------------------

Once you have the `extended.test` binary compiled and located on your system,
run:

```console
$ export KUBECONFIG=${KUBECONFIG-$HOME/.kube/config}
$ ./extended.test --ginko.focus="Mirror cluster"
```

After the command completes there will be a file created in the current
directory: `cm.yml`. This file can be then fed to the Cluster Loader command
below to recreate various cluster objects that were mirrored. 

The assumption for use with Cluster Loader is that the templates used in the
source cluster have same filename in the quickstarts subdirectory (of current
directory, relative to the cm.yml file).


# Cluster Loader

Cluster Loader can take user generated OpenShift application configurations
and load them onto an OpenShift cluster.

Running Cluster Mirror
----------------------

Once you have the `extended.test` binary compiled and located on your system,
run:

```console
$ export KUBECONFIG=${KUBECONFIG-$HOME/.kube/config}
$ ./extended.test --ginko.focus="Load cluster" --viper-config=config/test
```

After the execution completes the cluster will have deployed the ojects defined
in the configuration.

[OpenShift.com -- Using Cluster Loader](https://docs.openshift.com/container-platform/3.9/scaling_performance/using_cluster_loader.html)
