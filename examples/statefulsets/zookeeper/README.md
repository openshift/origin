# Zookeeper

This example runs zookeeper through a statefulset.

## Bootstrap

Create the statefulset in this directory
```
$ kubetl create -f zookeeper.yaml
```

Once you have all 3 nodes in Running, you can run the "test.sh" script in this directory.

## Failover

You can test failover by killing the leader. Insert a key:
```console
$ kubectl exec zoo-0 -- /opt/zookeeper/bin/zkCli.sh create /foo bar;
$ kubectl exec zoo-2 -- /opt/zookeeper/bin/zkCli.sh get /foo;

Watch existing members:
```console
$ kubectl run --attach bbox --image=busybox --restart=Never -- sh -c 'while true; do for i in 0 1 2; do echo zoo-$i $(echo stats | nc zoo-$i.zk:2181 | grep Mode); sleep 1; done; done';
zoo-2 Mode: follower
zoo-0 Mode: follower
zoo-1 Mode: leader
zoo-2 Mode: follower
```

Delete pets and wait for the statefulset controller to bring the back up:
```console
$ kubectl delete po -l app=zk
$ kubectl get po --watch-only
NAME      READY     STATUS     RESTARTS   AGE
zoo-0     0/1       Init:0/2   0          16s
zoo-0     0/1       Init:0/2   0         21s
zoo-0     0/1       PodInitializing   0         23s
zoo-0     1/1       Running   0         41s
zoo-1     0/1       Pending   0         0s
zoo-1     0/1       Init:0/2   0         0s
zoo-1     0/1       Init:0/2   0         14s
zoo-1     0/1       PodInitializing   0         17s
zoo-1     0/1       Running   0         18s
zoo-2     0/1       Pending   0         0s
zoo-2     0/1       Init:0/2   0         0s
zoo-2     0/1       Init:0/2   0         12s
zoo-2     0/1       Init:0/2   0         28s
zoo-2     0/1       PodInitializing   0         31s
zoo-2     0/1       Running   0         32s
...

zoo-0 Mode: follower
zoo-1 Mode: leader
zoo-2 Mode: follower
```

Check the previously inserted key:
```console
$ kubectl exec zoo-1 -- /opt/zookeeper/bin/zkCli.sh get /foo
ionid = 0x354887858e80035, negotiated timeout = 30000

WATCHER::

WatchedEvent state:SyncConnected type:None path:null
bar
```

## Scaling

You can scale up by modifying the number of replicas on the StatefulSet.

## Image Upgrade

TODO: Add details

## Maintenance

TODO: Add details

## Limitations
* Both statefulset and init containers are in alpha
* Look through the on-start and on-change scripts for TODOs
* Doesn't support the addition of observers through the statefulset
* Only supports storage options that have backends for persistent volume claims
