Replicated ZooKeeper Template
=============================

ZooKeeper is a distributed, open-source coordination service for distributed
applications. It exposes a simple set of primitives that distributed
applications can build upon to implement higher level services for
synchronization, configuration maintenance, and groups and naming. It is
designed to be easy to program to, and uses a data model styled after the
familiar directory tree structure of file systems. It runs in Java and has
bindings for both Java and C.

For more informations about ZooKeeper visit:

http://zookeeper.apache.org/doc/r3.4.6/zookeeperOver.html

### Pre-requires

You can 'pre-pull' the Docker image used by this template by:

```
$ docker pull openshift/zookeeper-346-fedora20
```

It is not really required, but it will speed up the launch of the pods.

### How to use this template

You can import this template to OpenShift using:

```
$ oc create -f examples/zookeeper/template.json
```

Then you can navigate to OpenShift UI and click the 'Create' button on top right
and choose 'Browse templates...'. Choose the 'zookeeper' and hit create.

Another way, is to use the CLI only:

```
$ oc process -f examples/zookeeper/template.json | oc create -f -
```

### How does it work

This template create three ZooKeeper pods that will act as three independent
servers. The ZooKeeper image is shipped with 'zoo.cfg' file that has these three
servers pre-configured. Once the pods are running, they will elect 'leader'.

There are four services in this template. The 'zookeeper' service serves as
entrypoint for clients to connect to ZooKeeper cluster. The other three services
are used for the leader election and for the sharing replicas across ZooKeeper
servers.

### Cleaning up

If you're done playing, you can remove all created resources by executing
following command:

```
$ ./examples/zookeeper/teardown.sh
```

Note: This will also remove all data you have stored in the ZooKeeper.
