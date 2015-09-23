Example 1: Using Local (hostPath) Storage
=========================================

This example uses the local storage directly on the OSE host where the mysql container is running. Make sure that any mysql pods created in other examples have been deleted before continuing with this example:

```
#on the OSE-master:
$ oc get pods

#if you see a mysql pod above:
$ oc delete pod <pod-name>
```

### Environment:
If the steps to install the environment, OSE, and mysql have not already been completed, then follow the instuctions linked-to directly below:

The enviromnent used for all of the examples here is described [here](../ENV.md).

### Setting up Openshift Enterprise (OSE):
The steps needed to setup a simple OSE cluster with 1 master and 1 worker node are described [here](../OSE.md).

### Setting up MySQL:
Follow the instructions [here](../MYSQL.md) to initialize and validate containerized mysql.

### Mysql Pod Spec File:
The [pod spec](mysql.yaml) uses a mysql image, defines the password as an environment variable, and maps the container's volume (/var/lib/mysql) to the target OSE-node's volume (/opt/mysql), where the database resides. Since selinux is enabled/enforcing, before the pod can be created, the local directory (/opt/mysql) on *each* schedulable OSE-node needs a selinux context label set to permit mysql to access the directory (via docker's bind mount):

```
#on *each* OSE-node:
$ getenforce
$ setenforce 1  #if selinux is permissive

$ mkdir -p /opt/mysql

$ chcon -Rt svirt_sandbox_file_t /opt/mysql
$ ls -Zd /opt/mysql/
drwxr-xr-x. polkitd ssh_keys system_u:object_r:svirt_sandbox_file_t:s0 /opt/mysql/
```

On the OSE-master, create the mysql pod via *oc create*:

```
$ oc create -f mysql.yaml 
pods/mysql

$ oc get pod
NAME                      READY     STATUS          RESTARTS   AGE
mysql                     1/1       Running         0          18s
```

To see which OSE host the mysql pod has been scheduled on:

```
#on the OSE-master:
$ oc describe pod mysql
NAME                      READY     STATUS                                                                                               RESTARTS   AGE
docker-registry-2-223nv   0/1       Image: registry.access.redhat.com/openshift3/ose-docker-registry:v3.0.1.0 is not ready on the node   0          3d
mysql                     1/1       Running                                                                                              0          18s
[root@rhel7-ose-1 ceph]# oc describe pod mysql
Name:				mysql
Namespace:			default
Image(s):			mysql
Node:				192.168.122.254/192.168.122.254  ## <--- the hostname is often shown in addition to ip
Labels:				name=mysql
Status:				Running
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

On the target (scheduled) OSE host, run docker to get information about the mysql container:

```
#on the target OSE-node:
$ docker ps
CONTAINER ID        IMAGE                         COMMAND                CREATED             STATUS              PORTS               NAMES
77f4af567e3d        mysql                         "/entrypoint.sh mysq   5 minutes ago       Up 5 minutes                            k8s_mysql.4977675e_mysql_default_ea9b64de-5290-11e5-b56b-52540039f12e_2257d0b5   
dca749fa3530        openshift3/ose-pod:v3.0.1.0   "/pod"                 5 minutes ago       Up 5 minutes                            k8s_POD.892ec37e_mysql_default_ea9b64de-5290-11e5-b56b-52540039f12e_aa534a81

$ docker inspect mysql
[{
    "Id": "7eee2d462c8f6ffacfb908cc930559e21778f60afdb2d7e9cf0f3025274d7ea8",
...
    "ContainerConfig": {
        "Hostname": "c6ebf900c860",
...
        "ExposedPorts": {
            "3306/tcp": {}
        },
...
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
...
    },
    "DockerVersion": "1.7.1",
...
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
...
    },
...
}]

$ docker logs 77f4af567e3d   # <--- container ID
2015-09-03 23:10:17 0 [Note] mysqld (mysqld 5.6.26) starting as process 1 ...
...
2015-09-03 23:10:18 1 [Note] InnoDB: 5.6.26 started; log sequence number 1626017
2015-09-03 23:10:18 1 [Note] Server hostname (bind-address): '*'; port: 3306
...
2015-09-03 23:10:18 1 [Note] mysqld: ready for connections.
Version: '5.6.26'  socket: '/var/run/mysqld/mysqld.sock'  port: 3306  MySQL Community Server (GPL)
```

Finally, on the same OSE-node, run mysql inside the container:

```
$ docker exec -it 77f4af567e3d bash  # <--- container ID again
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
