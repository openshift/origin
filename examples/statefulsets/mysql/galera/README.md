# Mysql Galera

This example runs mysql galera through a statefulset.

## Bootstrap

Create the statefulset in this directory
```
$ kubectl create -f galera.yaml
```

Once you have all 3 nodes in Running, you can run the "test.sh" script in this directory.
This example requires manual intervention.
Once you have all 3 nodes in Running, you can run the "test.sh" script in this directory.

## Caveats

Starting up all galera nodes at once leads to an issue where all the mysqls
believe they're in the primary component because they don't see the others in
the DNS. For the bootstrapping to work: mysql-0 needs to see itself, mysql-1
needs to see itself and mysql-0, and so on, because the first node that sees
a peer list of 1 will assume it's the leader.

## TODO

Expect better solutions for the following as statefulset matures.

* Failover
* Scaling Up
* Scaling Down
* Image Upgrade
* Maintenance
