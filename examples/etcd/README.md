Clustered etcd Template
========================

etcd is a distributed, consistent key value store for shared configuration and
service discovery. For more details about etcd, visit:

https://github.com/coreos/etcd

### Requirements

You can 'pre-pull' the Docker image used by this template by:

```
$ docker pull openshift/etcd-20-centos7
```

You can also build this Docker image yourself, by using provided Makefile:

```
$ make
```

### How to use this template

You can import this template to OpenShift using:

```
$ oc create -f examples/etcd/template.json
```

Then you can navigate to OpenShift UI and click the 'Create' button on top right
and choose 'Browse templates...'. Choose the 'etcd' and hit create.

Another way, is to use the CLI only:

```
$ oc process -f examples/etcd/template.json | oc create -f -
```

### How does it work

This template creates two Services. The first service is used for initial
discovery and stores information about running members. This service is used
only internally by the cluster members and you should not need to access it.
You can however obtain information about the current state/size of the cluster.

The second service 'etcd' is the main entrypoint for accessing the 'etcd'
cluster. This service is exposing two ports. The port 2380 is used for internal
server-to-server communication and the port 2379 is used for the client
connections.

The 'etcd-discovery' pod created by this template will create an instance of
etcd server, that is used as cluster discovery service. This pod can be stopped
or deleted when desired size of the cluster is reached. If you want to add more
members you will have to start this pod again manually.

The 'etcd' replication controller manage creation of the etcd cluster members.
By default this template will start 3 members. The members then register
themselves using the discovery service and elect the leader. You can adjust the
number of replicas as long as the 'etcd-discovery' service is running.

### Cleaning up

If you're done playing, you can remove all the created resources by executing
following command:

```
$ ./examples/etcd/teardown.sh
```

Note: This will also remove all data you have stored in the etcd.
