Example 4: MySQL + PVClaim in Single OSE Template
=================================================

Here is an example showing how to create the mysql application using ceph rbd as the persistent store, where the pod and persistent volume claim (PVC) are both defined in one OSE template file. Make sure that any mysql pods created in other examples have been deleted before continuing with this example:

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

### Defining the Template File:
Here is the [template file](ceph-mysql-template.yaml) which defines both the persistent volume claim (PVC) and the pod. The actual persistent volume (PV) has already been created in [example 3](../mysql_ceph_pvc), and is verified here. Note, as in the other examples, the mysql pod/container needs to run privileged, which is specified in the "container:" portion of the template.

```
#on the OSE-master:
$ oc get pv
NAME                 LABELS    CAPACITY     ACCESSMODES   STATUS    CLAIM                   REASON
ceph-pv              <none>    2147483648   RWX           Bound     default/ceph-claim 
```

### Create the Template Object:

```
#on the OSE-master:
$ oc create -f ceph-mysql-template.yaml 
templates/ceph-mysql-template

# oc get templates
NAME                  DESCRIPTION                                                   PARAMETERS    OBJECTS
ceph-mysql-template   mysql persistent ceph application template using inline PVC   0 (all set)   2
```

Note: when re-using this template, if the pvc already exists, you'll see this error: *Error: persistentvolumeclaims "ceph-claim-template" already exists*. This error can be ignored since the pod is still started correctly.

### PVC Cleanup:
Before the mysql app can be created from the above template, we need to make sure there is a PV available for the defined template claim. The *oc get pv* command above shows that the only PV we have is Bound to the "ceph-claim", which we created in [example 3](../mysql_ceph_pvc). Therefore, we first need to delete the current PVC:

```
#on the OSE-master:
$ oc delete pvc ceph-claim
persistentvolumeclaims/ceph-claim

# oc get pvc
NAME            LABELS    STATUS    VOLUME

# oc get pv
NAME                 LABELS    CAPACITY     ACCESSMODES   STATUS     CLAIM                   REASON
ceph-pv              <none>    2147483648   RWX           Released   default/ceph-claim 
```

Notice that the "ceph-claim" is gone and that the "ceph-pv" is Released. This may be a bug, but currently a new claim cannot bind to a Released PV; therefore, we have to also delete and recreate the PV before we can start mysql from the template.

```
#on the OSE-master:
$ oc delete pv 
persistentvolumes/ceph-pv

$ oc create -f ceph-pv.yaml
persistentvolumes/ceph-pv

$ oc get pv
NAME                 LABELS    CAPACITY     ACCESSMODES   STATUS      CLAIM                   REASON
ceph-pv              <none>    2147483648   RWX           Available                           

```
The pv file used above is defined [here](../mysql_ceph_pvc/ceph-pv.yaml).

### Create the Mysql Pod:
The *oc new-app* command, which accepts a template object (or a template file can be specified), is used to create the mysql app from this template:

```
#on the OSE-master:
$ oc new-app ceph-mysql-template
persistentvolumeclaims/ceph-claim-template
pods/ceph-mysql-pod
Run 'oc status' to view your app.
```

Check on the PV, PVC and pod:

```
#on the OSE-master:
$ oc get pvc
NAME                  LABELS    STATUS    VOLUME
ceph-claim-template   map[]     Bound     ceph-pv

$ oc get pv
NAME                 LABELS    CAPACITY     ACCESSMODES   STATUS    CLAIM                         REASON
ceph-pv              <none>    2147483648   RWX           Bound     default/ceph-claim-template 

$ oc get pods
NAME                      READY     STATUS  RESTARTS   AGE
ceph-mysql-pod            1/1       Running 0          25s

$ oc describe pod ceph-mysql-pod
Name:				ceph-mysql-pod
Namespace:			default
Image(s):			mysql
Node:				192.168.122.254/192.168.122.254
Labels:				<none>
Status:				Running
Reason:				
Message:			
IP:				10.1.0.44
Replication Controllers:	<none>
Containers:
  mysql-from-template:
    Image:		mysql
    State:		Running
      Started:		Fri, 04 Sep 2015 16:04:11 -0400
    Ready:		True
    Restart Count:	0
Conditions:
  Type		Status
  Ready 	True 
Events:
  FirstSeen				LastSeen			Count	From				SubobjectPath				Reason	Message
  Fri, 04 Sep 2015 15:39:41 -0400	Fri, 04 Sep 2015 15:39:41 -0400	1	{scheduler }								scheduled	Successfully assigned ceph-mysql-pod to 192.168.122.254

  Fri, 04 Sep 2015 16:04:04 -0400	Fri, 04 Sep 2015 16:04:04 -0400	1	{kubelet 192.168.122.254}	implicitly required container POD	pulled	Pod container image "openshift3/ose-pod:v3.0.1.0" already present on machine
  Fri, 04 Sep 2015 16:04:07 -0400	Fri, 04 Sep 2015 16:04:07 -0400	1	{kubelet 192.168.122.254}	implicitly required container POD	createdCreated with docker id 5d310cd5ddc2
  Fri, 04 Sep 2015 16:04:07 -0400	Fri, 04 Sep 2015 16:04:07 -0400	1	{kubelet 192.168.122.254}	implicitly required container POD	startedStarted with docker id 5d310cd5ddc2
  Fri, 04 Sep 2015 16:04:11 -0400	Fri, 04 Sep 2015 16:04:11 -0400	1	{kubelet 192.168.122.254}	spec.containers{mysql-from-template}	createdCreated with docker id 183be9a22a13
  Fri, 04 Sep 2015 16:04:11 -0400	Fri, 04 Sep 2015 16:04:11 -0400	1	{kubelet 192.168.122.254}	spec.containers{mysql-from-template}	startedStarted with docker id 183be9a22a13
```

On the target OSE node we can verify that msql is working:

```
#on the scheduled/target OSE-node:
$ docker ps
CONTAINER ID        IMAGE                         COMMAND                CREATED             STATUS              PORTS               NAMES
183be9a22a13        mysql                         "/entrypoint.sh mysq   5 minutes ago       Up 5 minutes                            k8s_mysql-from-template.b4384d92_ceph-mysql-pod_default_af388e69-533c-11e5-b56b-52540039f12e_faf1cdc0   
5d310cd5ddc2        openshift3/ose-pod:v3.0.1.0   "/pod"                 5 minutes ago       Up 5 minutes                            k8s_POD.892ec37e_ceph-mysql-pod_default_af388e69-533c-11e5-b56b-52540039f12e_ede13b32 
Using the above container ID, we can inspect the docker logs, and then run a shell inside this container to show the ceph/rbd mount and to access a simple database (previously created), as follows:
```

And, as in other examples, we can shell into the running mysql container and execute mysql:

```
$ docker exec -it 183be9a22a13 bash
root@ceph-mysql-pod:/# ls /var/lib/mysql/
ib_logfile0  ibdata1     mysql               us_states
ib_logfile1  lost+found  performance_schema

root@ceph-mysql-pod:/# mysql
Welcome to the MySQL monitor.  Commands end with ; or \g.
Your MySQL connection id is 2
Server version: 5.5.44-0ubuntu0.14.04.1 (Ubuntu)
...
Type 'help;' or '\h' for help. Type '\c' to clear the current input statement.

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

mysql> show tables;
+---------------------+
| Tables_in_us_states |
+---------------------+
| states              |
+---------------------+
1 row in set (0.00 sec)

mysql> select * from states;
+----+---------+------------+
| id | state   | population |
+----+---------+------------+
|  1 | Alabama |    4822023 |
+----+---------+------------+
1 row in set (0.00 sec)

mysql> quit
Bye
root@ceph-mysql-pod:/# exit
exit
```
