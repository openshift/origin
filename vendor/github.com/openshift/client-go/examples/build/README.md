# Build operations

This example program demonstrates the fundamental operations for working
with
[Build](https://docs.openshift.com/container-platform/3.7/dev_guide/builds/basic_build_operations.html)
resources, such as `List builds`, `Retrieve details of a build` and `Trigger a build`.

## Running this example

Make sure you have an OpenShift cluster and `oc` is configured:

```sh
$ oc get nodes
### Create a project
$ oc new-project testproject
### Create an app
$ oc new-app https://github.com/sclorg/cakephp-ex
$ oc get build
NAME           TYPE      FROM          STATUS     STARTED          DURATION
cakephp-ex-1   Source    Git@e04b8cc   Complete   37 minutes ago   1m39s
```

Compile this example on your workstation:

```sh
$ go build -o examples/build/app ./examples/build/
```

Now, run this application on your workstation with your local kubeconfig file:

```sh
$ ./examples/build/app
```