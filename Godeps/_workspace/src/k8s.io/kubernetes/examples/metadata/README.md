<!-- BEGIN MUNGE: UNVERSIONED_WARNING -->

<!-- BEGIN STRIP_FOR_RELEASE -->

<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">
<img src="http://kubernetes.io/img/warning.png" alt="WARNING"
     width="25" height="25">

<h2>PLEASE NOTE: This document applies to the HEAD of the source tree</h2>

If you are using a released version of Kubernetes, you should
refer to the docs that go with that version.

<strong>
The latest 1.0.x release of this document can be found
[here](http://releases.k8s.io/release-1.0/examples/metadata/README.md).

Documentation for other releases can be found at
[releases.k8s.io](http://releases.k8s.io).
</strong>
--

<!-- END STRIP_FOR_RELEASE -->

<!-- END MUNGE: UNVERSIONED_WARNING -->

# Metadata volume plugin

Following this example, you will create a pod with a metadata volume.
A metadata volume is a k8s volume plugin with the ability to save pod [metadata](../../docs/devel/api-conventions.md#metadata) field to a plain text file.

Supported metadata fields:

1. `metadata.annotations`
2. `metadata.namespace`
3. `metadata.name`
4. `metadata.labels`

### Step Zero: Prerequisites

This example assumes you have a Kubernetes cluster installed and running, and the ```kubectl``` command line tool somewhere in your path. Please see the [gettingstarted](../../docs/getting-started-guides/) for installation instructions for your platform.

### Step One: Create the pod

Use the `examples/metadata/metadata-volume.yaml` file to create a Pod with a  volume plugin which stores pod labels and pod annotations to `/etc/labels` and  `/etc/annotations` respectively.

```shell
$ kubectl create -f  examples/metadata/metadata-volume.yaml
```

### Step Two: Examine pod/container output

The pod displays (every 5 seconds) the content of the dump files which can be executed via the usual `kubectl log` command

```shell
$ kubectl logs kubernetes-metadata-volume-example
cluster=test-cluster1
rack=rack-22
zone=us-est-coast
build=two
builder=john-doe
kubernetes.io/config.source=api
```

### Internals

In pod's `/etc` directory one may find the metadata created by the plugin (system files elided):

```shell
$ kubectl exec kubernetes-metadata-volume-example -i -t -- sh
/ # ls -laR /etc
/etc:
total 16
drwxrwxrwt    3 0        0              180 Jun  2 21:01 .
drwxr-xr-x    1 0        0             4096 Jun  2 21:01 ..
drwx------    2 0        0               80 Jun  2 21:01 .2015_06_02_21_01_10575342153
lrwxrwxrwx    1 0        0               29 Jun  2 21:01 .current -> .2015_06_02_21_01_10575342153
lrwxrwxrwx    1 0        0               20 Jun  2 21:01 annotations -> .current/annotations
lrwxrwxrwx    1 0        0               15 Jun  2 21:01 labels -> .current/labels

/etc/.2015_06_02_21_01_10575342153:
total 8
drwx------    2 0        0               80 Jun  2 21:01 .
drwxrwxrwt    3 0        0              180 Jun  2 21:01 ..
-rw-r--r--    1 0        0               59 Jun  2 21:01 annotations
-rw-r--r--    1 0        0               53 Jun  2 21:01 labels
/ #
```

Metadata is stored in a temporary directory (`.2015_06_02_21_01_10575342153` in the example above) which is symlinked to by `.current`. Symlinks for annotations and labels in `/etc` point to files containing the actual metadata through the `.current` indirection.  This structure allows for dynamic atomic refresh of the metadata: updates are written to a new temporary directory, and the `.current` symlink is updated atomically using `rename(2)`.


<!-- BEGIN MUNGE: GENERATED_ANALYTICS -->
[![Analytics](https://kubernetes-site.appspot.com/UA-36037335-10/GitHub/Godeps/_workspace/src/k8s.io/kubernetes/examples/metadata/README.md?pixel)]()
<!-- END MUNGE: GENERATED_ANALYTICS -->
