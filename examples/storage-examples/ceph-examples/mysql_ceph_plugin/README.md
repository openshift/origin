Example 2: Using the Ceph-RBD Plugin
====================================

This example uses the ceph/rbd plugin directly in the pod spec. Make sure that any mysql pods created in other examples have been deleted before continuing with this example:

```
#on the OSE-master:
$ oc get pods

#if you see a mysql pod above:
$ oc delete pod <pod-name>
```

### Environment:
If the steps to install the environment, ceph, ose, and mysql have not already been completed, then follow the instuctions linked-to directly below:

The enviromnent used for all of the examples is described [here](../ENV.md). It is assumed that ceph is already up and running, either on bare metal, in a VM, or containerized.

### Setting up Openshift Enterprise (OSE):
The steps needed to setup a simple OSE cluster with 1 master and 1 worker node are described [here](../OSE.md).

### Setting up MySQL:
Follow the instructions [here](../MYSQL.md) to initialize and validate containerized mysql.

### Defining the Pod Spec File:
This example defines a simple [pod spec](mysql-ceph-pod.yaml) using the same mysql image, a container volume mount which references the ceph-rbd persistent storage plugin, and the ceph monitor's ip address.

The pod created by this spec needs to be **privileged**, as seen in the container's "securityContext" section. This is necessary for the rbd mount to succeed. If the container is not privileged and if the OSE-node has selinux set to *enforcing* then this error will be seen:

```
#on the target OSE-node:
$ docker logs 45f023ef10ce  #mysql container-ID
chown: cannot read directory `/var/lib/mysql/': Permission denied
```

By setting selinux to permissive on *each* OSE-node the above error is also aboided. However, running the mysql container privileged is likely preferred.

On the OSE-master, create the mysql pod:

```
#on the OSE-master:
$ oc create -f mysql-ceph-pod.yaml 
pods/mysql

$ oc get pod
NAME                      READY     STATUS          RESTARTS   AGE
mysql                     1/1       Running         0          18s
```

To see volume information for this pod:

```
#on the OSE-master:
$ oc volume pod mysql --list
# pods mysql, volumes:
mysql-ceph
default-token-sqef4
	# container mysql, volume mounts:
	mysql-ceph /var/lib/mysql
	default-token-sqef4 /var/run/secrets/kubernetes.io/serviceaccount

```
"mysql-ceph" is the volume name defined in the pod spec.

To see which OSE host the mysql pod has been scheduled on:

```
#on the OSE-master:
$ oc describe pod mysql
Name:				mysql
Namespace:	default
Image(s):		mysql
Node:				192.168.122.254/192.168.122.254  ## <--- the hostname is often shown here
Labels:			name=mysql
Status:			Running
Reason:				
Message:			
IP:				10.1.0.41
Replication Controllers:	<none>
Containers:
  mysql:
    Image:		mysql
    State:		Running
      Started:		Thu, 03 Sep 2015 19:10:15 -0400
    Ready:		True
    Restart Count:	0
Conditions:
  Type		Status
  Ready 	True 
Events:
  FirstSeen				LastSeen			Count	From				SubobjectPath				Reason	Message
  Thu, 03 Sep 2015 19:10:07 -0400	Thu, 03 Sep 2015 19:10:07 -0400	1	{scheduler }								scheduled	Successfully assigned mysql to 192.168.122.254
...
  Thu, 03 Sep 2015 19:10:15 -0400	Thu, 03 Sep 2015 19:10:15 -0400	1	{kubelet 192.168.122.254}	spec.containers{mysql}			startedStarted with docker id 77f4af567e3d
```

On the target OSE-node use docker to get information about the mysql container:

```
$ docker ps
CONTAINER ID        IMAGE                         COMMAND                CREATED             STATUS              PORTS               NAMES
77f4af567e3d        mysql                         "/entrypoint.sh mysq   5 minutes ago       Up 5 minutes                            k8s_mysql.4977675e_mysql_default_ea9b64de-5290-11e5-b56b-52540039f12e_2257d0b5   
dca749fa3530        openshift3/ose-pod:v3.0.1.0   "/pod"                 5 minutes ago       Up 5 minutes                            k8s_POD.892ec37e_mysql_default_ea9b64de-5290-11e5-b56b-52540039f12e_aa534a81

$ docker inspect mysql
[{
    "Id": "7eee2d462c8f6ffacfb908cc930559e21778f60afdb2d7e9cf0f3025274d7ea8",
    "Parent": "15a3cddfc178c4dbaa8f56142d4eebef6d22a3cd1842820844cf815992fe5a13",
    "Comment": "",
    "Created": "2015-08-24T21:55:13.704277966Z",
    "Container": "113d16e2420bb0f5e17a74f5f8c85d70572efe1720451da83e0a84fe3fcd04fd",
    "ContainerConfig": {
        "Hostname": "c6ebf900c860",
        "Domainname": "",
        "User": "",
        "AttachStdin": false,
        "AttachStdout": false,
        "AttachStderr": false,
        "PortSpecs": null,
        "ExposedPorts": {
            "3306/tcp": {}
        },
        "Tty": false,
        "OpenStdin": false,
        "StdinOnce": false,
        "Env": [
            "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
            "MYSQL_MAJOR=5.6",
            "MYSQL_VERSION=5.6.26"
        ],
        "Cmd": [
            "/bin/sh",
            "-c",
            "#(nop) CMD [\"mysqld\"]"
        ],
        "Image": "15a3cddfc178c4dbaa8f56142d4eebef6d22a3cd1842820844cf815992fe5a13",
        "Volumes": {
            "/var/lib/mysql": {}
        },
        "VolumeDriver": "",
        "WorkingDir": "",
        "Entrypoint": [
            "/entrypoint.sh"
        ],
        "NetworkDisabled": false,
        "MacAddress": "",
        "OnBuild": [],
        "Labels": {},
        "Init": ""
    },
    "DockerVersion": "1.7.1",
    "Author": "",
    "Config": {
        "Hostname": "c6ebf900c860",
        "Domainname": "",
        "User": "",
        "AttachStdin": false,
        "AttachStdout": false,
        "AttachStderr": false,
        "PortSpecs": null,
        "ExposedPorts": {
            "3306/tcp": {}
        },
        "Tty": false,
        "OpenStdin": false,
        "StdinOnce": false,
        "Env": [
            "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
            "MYSQL_MAJOR=5.6",
            "MYSQL_VERSION=5.6.26"
        ],
        "Cmd": [
            "mysqld"
        ],
        "Image": "15a3cddfc178c4dbaa8f56142d4eebef6d22a3cd1842820844cf815992fe5a13",
        "Volumes": {
            "/var/lib/mysql": {}
        },
        "VolumeDriver": "",
        "WorkingDir": "",
        "Entrypoint": [
            "/entrypoint.sh"
        ],
        "NetworkDisabled": false,
        "MacAddress": "",
        "OnBuild": [],
        "Labels": {},
        "Init": ""
    },
    "Architecture": "amd64",
    "Os": "linux",
    "Size": 0,
    "VirtualSize": 283575255
}]

$ docker logs 77f4af567e3d   # <--- container ID
2015-09-03 23:10:17 0 [Note] mysqld (mysqld 5.6.26) starting as process 1 ...
2015-09-03 23:10:17 1 [Note] Plugin 'FEDERATED' is disabled.
2015-09-03 23:10:17 1 [Note] InnoDB: Using atomics to ref count buffer pool pages
2015-09-03 23:10:17 1 [Note] InnoDB: The InnoDB memory heap is disabled
2015-09-03 23:10:17 1 [Note] InnoDB: Mutexes and rw_locks use GCC atomic builtins
2015-09-03 23:10:17 1 [Note] InnoDB: Memory barrier is not used
2015-09-03 23:10:17 1 [Note] InnoDB: Compressed tables use zlib 1.2.7
2015-09-03 23:10:17 1 [Note] InnoDB: Using Linux native AIO
2015-09-03 23:10:17 1 [Note] InnoDB: Using CPU crc32 instructions
2015-09-03 23:10:17 1 [Note] InnoDB: Initializing buffer pool, size = 128.0M
2015-09-03 23:10:17 1 [Note] InnoDB: Completed initialization of buffer pool
2015-09-03 23:10:17 1 [Note] InnoDB: Highest supported file format is Barracuda.
2015-09-03 23:10:18 1 [Note] InnoDB: 128 rollback segment(s) are active.
2015-09-03 23:10:18 1 [Note] InnoDB: Waiting for purge to start
2015-09-03 23:10:18 1 [Note] InnoDB: 5.6.26 started; log sequence number 1626017
2015-09-03 23:10:18 1 [Note] Server hostname (bind-address): '*'; port: 3306
2015-09-03 23:10:18 1 [Note] IPv6 is available.
2015-09-03 23:10:18 1 [Note]   - '::' resolves to '::';
2015-09-03 23:10:18 1 [Note] Server socket created on IP: '::'.
2015-09-03 23:10:18 1 [Warning] 'proxies_priv' entry '@ root@mysql' ignored in --skip-name-resolve mode.
2015-09-03 23:10:18 1 [Note] Event Scheduler: Loaded 0 events
2015-09-03 23:10:18 1 [Note] mysqld: ready for connections.
Version: '5.6.26'  socket: '/var/run/mysqld/mysqld.sock'  port: 3306  MySQL Community Server (GPL)
```

Finally, on the same OSE-node, run mysql inside the container:

```
$ docker exec -it 77f4af567e3d bash  # <--- mysql container ID
root@mysql:/# mysql -p
Enter password: 
Welcome to the MySQL monitor.  Commands end with ; or \g.
...
Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.

mysql> quit
Bye
root@mysql:/# exit
exit
```
