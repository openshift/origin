# Cluster Mirror

This document describes how to use the Cluster Mirror to create a Cluster
Loader configuration of a local OpenShift cluster. Specifically the templates
and pods that are contained in the various cluster namespaces.

The cluster mirror tool was conceived to provide a way to reproduce (with
reasonably approximate fidelity) a control-plane workload. The idea is to make
sure our scale testing efforts are as realistic as possible.

Use-cases include:

 * Replicating OpenShift Online clusters into lab environments for scale testing and R&D.
 * Reproducing customer environments in support situations.

Note: This tool does not inspect persistent storage, it does not look at the
content of secrets and it does not export user data in any way. It takes a
"fingerprint" of the target environment via API calls, and allows users to
"replay" that environment elsewhere (in terms of number of namespaces,
templates and pods and what type they are).

Running Cluster Mirror
----------------------

Cluster Loader is implemented as an extended test in OpenShift, and thus is is
distributed as a precompiled binary in the atomic-openshift-tests RPM.

Once you have the `extended.test` binary installed on your system, run:

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

Running Cluster Loader
----------------------

Cluster Loader is implemented as an extended test in OpenShift, and thus is is
distributed as a precompiled binary in the atomic-openshift-tests RPM.

Once you have the `extended.test` binary installed on your system, run:

```console
$ export KUBECONFIG=${KUBECONFIG-$HOME/.kube/config}
$ ./extended.test --ginko.focus="Load cluster" --viper-config=config/test
```

After the execution completes the cluster will have deployed the ojects defined
in the configuration.

[OpenShift.com -- Using Cluster Loader](https://docs.openshift.com/container-platform/3.9/scaling_performance/using_cluster_loader.html)
