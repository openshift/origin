Example 3: Using MySQL, Ceph Persistent Volume and Claim
========================================================

Here is an example of how to create the mysql application, using ceph rbd as the persistent store, where the ceph rbd is defined in a persistent volume (PV), and the pod uses a persistent volume claim (PVC), rather than defining the rbd volume inline. The kubernetetes PVClaimBinder matches the pod's claim against PVs and binds the claim to the PV that has the best match. Today, the matching criteria is very simple -- capacity and sharing attributes -- but, hopefully richer PV and PVC definitions will be coming in the future.  See  the [kubernetes persistent storage](https://github.com/kubernetes/kubernetes/blob/master/docs/design/persistent-storage.md) document for more information on PVs and PVCs.

Make sure that any mysql pods created in other examples have been deleted before continuing with this example:

```
#on the OSE-master:
$ oc get pods

#if you see a mysql pod above:
$ oc delete pod <pod-name>
```

### Environment:
If the steps to install the environment, ceph, ose and mysql have not already been completed, then follow the instuctions linked-to directly below:

The enviromnent used for all of the examples is described [here](../ENV.md). It is assumed that ceph is already up and running, either on bare metal, in a VM, or containerized.

### Setting up Openshift Enterprise (OSE):
The steps needed to seup a simple OSE cluster with 1 master and 1 worker node are described [here](../OSE.md).

### Setting up MySQL:
Follow the instructions [here](../MYSQL.md) to initialize and validate containerized mysql.

### Defining the PV and PVC Files:
A persistent volume is created from a file defining the name, capacity, and access methods for persistent storage. The PV spec used in this example is [here](ceph-pv.yaml), and the PVC is [here](ceph-claim.yaml).

PVs are typically created by an OSE administrator, whereas PVCs will typically be created and requested by non-admins. The example here creates both the PV and claim separate from the pod. There is also a [template](../mysql_ceph_template) example which defines the PVC in the same file used to define the pod.

### Creating the PV and PVC:
*oc create -f* is execute on the OSE-master to create almost all OSE objects and is used here to create the ceph PV and PVC.

```
#on the OSE-master:
$ oc create -f ceph-pv.yaml
persistentvolumes/ceph-pv

$ oc get pv
NAME                 LABELS    CAPACITY     ACCESSMODES   STATUS      CLAIM                   REASON
ceph-pv              <none>    2147483648   RWX           Available             

$ oc create -f ceph-claim.yaml
persistentvolumeclaims/ceph-claim

$ oc get pvc
NAME            LABELS    STATUS    VOLUME
ceph-claim      map[]     Bound     ceph-pv
```

Notice that the claim has been bound to the "ceph-pv" persistent volume.

### Creating the Pod:
The [pod spec](ceph-mysql-pvc-pod.yaml) references the same mysql image and defines the named claim to be used for persistent storage. As with [example 2](../mysql_ceph_plugin), the mysql container needs to run privileged. *oc create* is used to create the pod:

```
#on the OSE-master:
$ oc create -f ceph-mysql-pvc-pod.yaml 
pods/ceph-mysql

$ oc get pods
NAME                      READY     STATUS       RESTARTS   AGE
ceph-mysql                1/1       Running                                      
```

Volume information is also visible on the OSE-master:

```
#on the OSE-master:
$ oc volume pod ceph-mysql --list
# pods ceph-mysql, volumes:
mysql-pv
default-token-sqef4
	# container ceph-mysql, volume mounts:
	mysql-pv /var/lib/mysql
	default-token-sqef4 /var/run/secrets/kubernetes.io/serviceaccount
```

We execute *oc describe pod* to see which OSE host the pod is running on, and to see the pod's recent events:

```
#on the OSE-master:
$ oc describe pod ceph-mysql
Name:				ceph-mysql
Namespace:			default
Image(s):			mysql
Node:				192.168.122.254/192.168.122.254  # <--- often both hostname and ip are shown here
Labels:				<none>
Status:				Running
Reason:				
Message:			
IP:				10.1.0.43
Replication Controllers:	<none>
Containers:
  ceph-mysql:
    Image:		mysql
    State:		Running
      Started:		Fri, 04 Sep 2015 14:47:31 -0400
    Ready:		True
    Restart Count:	0
Conditions:
  Type		Status
  Ready 	True 
Events:
  FirstSeen				LastSeen			Count	From				SubobjectPath				Reason	Message
  Fri, 04 Sep 2015 14:47:22 -0400	Fri, 04 Sep 2015 14:47:22 -0400	1	{scheduler }								scheduled	Successfully assigned ceph-mysql to 192.168.122.254
  Fri, 04 Sep 2015 14:47:24 -0400	Fri, 04 Sep 2015 14:47:24 -0400	1	{kubelet 192.168.122.254}	implicitly required container POD	pulled	Pod container image "openshift3/ose-pod:v3.0.1.0" already present on machine
  Fri, 04 Sep 2015 14:47:26 -0400	Fri, 04 Sep 2015 14:47:26 -0400	1	{kubelet 192.168.122.254}	implicitly required container POD	createdCreated with docker id acbee2db9a18
  Fri, 04 Sep 2015 14:47:27 -0400	Fri, 04 Sep 2015 14:47:27 -0400	1	{kubelet 192.168.122.254}	implicitly required container POD	startedStarted with docker id acbee2db9a18
  Fri, 04 Sep 2015 14:47:30 -0400	Fri, 04 Sep 2015 14:47:30 -0400	1	{kubelet 192.168.122.254}	spec.containers{ceph-mysql}		createdCreated with docker id 9a43017dbebf
  Fri, 04 Sep 2015 14:47:31 -0400	Fri, 04 Sep 2015 14:47:31 -0400	1	{kubelet 192.168.122.254}	spec.containers{ceph-mysql}		startedStarted with docker id 9a43017dbebf
```

We see that the pod was scheduled on OSE host 192.168.122.254 (often there is a hostname visible too). On the target OSE node verify that the mysql container is running and that it's using the ceph-rbd volume:

```
#on the target/scheduled OSE-node:
$ docker ps
CONTAINER ID        IMAGE                         COMMAND                CREATED             STATUS              PORTS               NAMES
9a43017dbebf        mysql                         "/entrypoint.sh mysq   4 minutes ago       Up 4 minutes                            k8s_ceph-mysql.d8cb6e3e_ceph-mysql_default_608af6aa-5335-11e5-b56b-52540039f12e_b0163abc   
acbee2db9a18        openshift3/ose-pod:v3.0.1.0   "/pod"                 4 minutes ago       Up 4 minutes                            k8s_POD.dbbbe7c7_ceph-mysql_default_608af6aa-5335-11e5-b56b-52540039f12e_d030f08f 
```

The mysql container ID is 9a43017dbebf. More details on this container are availble via *docker inspect container-ID-or-name*. Log information is shown via *docker logs container-ID*, and via the *systemctl status openshift-node -l* and *journalctl -xe -u openshift-node docker* commands.

The container's rbd mounts are visible directly from the host and from within the container itself. On the OSE host:

```
#on the target/scheduled OSE-node:
$ mount | grep rbd
/dev/rbd0 on /var/lib/openshift/openshift.local.volumes/plugins/kubernetes.io/rbd/rbd/rbd-image-foo type ext4 (rw,relatime,seclabel,stripe=1024,data=ordered)
/dev/rbd0 on /var/lib/openshift/openshift.local.volumes/pods/608af6aa-5335-11e5-b56b-52540039f12e/volumes/kubernetes.io~rbd/ceph-pv type ext4 (rw,relatime,seclabel,stripe=1024,data=ordered)
```

And, shelling into the container:

```
#on the target/scheduled OSE-node:
$ docker exec -it 9a43017dbebf bash
root@ceph-mysql:/# mount | grep rbd
/dev/rbd0 on /var/lib/mysql type ext4 (rw,relatime,seclabel,stripe=1024,data=ordered)
root@ceph-mysql:/# exit
exit
```

Mysql can also be run in the container as follows:

```
#on the target/scheduled OSE-node:
$ docker exec -it 9a43017dbebf bash
root@ceph-mysql:/# mysql                                                       
Welcome to the MySQL monitor.  Commands end with ; or \g.
...
mysql> show databases;
+---------------------+
| Database            |
+---------------------+
| information_schema  |
| #mysql50#lost+found |
| mysql               |
| performance_schema  |
| us_states           |
+---------------------+
5 rows in set (0.00 sec)

mysql> use us_states;
Reading table information for completion of table and column names
...
mysql> select * from states;
+----+---------+------------+
| id | state   | population |
+----+---------+------------+
|  1 | Alabama |    4822023 |
+----+---------+------------+
1 row in set (0.00 sec)

mysql> quit
Bye
root@ceph-mysql:/# exit
exit
```
