Setup a Simple OpenShift Enterprise (OSE) Cluster
=================================================

### Environment:
The enviromnent used for all of the ceph examples is described [here](ENV.md). It is assumed that ceph is already up and running, either on bare metal, in a VM, or containerized.

### Setting up OSE:
The following OpenShift/origin guides should be followed when setting up OSE:
* https://docs.openshift.com/enterprise/3.0/admin_guide/install/prerequisites.html -- Prerequisites
* https://docs.openshift.org/latest/getting_started/administrators.html#running-in-a-docker-container -- Getting Started for Administrator
* https://docs.openshift.com/enterprise/3.0/admin_guide/install/quick_install.html -- **Quick Installation**
* https://docs.openshift.com/enterprise/3.0/admin_guide/configuring_authentication.html -- Configuring Authentication
* https://docs.openshift.com/enterprise/3.0/admin_guide/install/advanced_install.html -- Advanced Installation

The examples in here use "Method 1: Running the Installation Utility From the Internet" described in the [Quick Installation](https://docs.openshift.com/enterprise/3.0/admin_guide/install/quick_install.html) Guide above.

The following checks can be made to ensure that OSE is installed correctly and is running:
* is the OSE master server is running? On the OSE-master host, execute *systemctl status openshift-master*. Use *systemctl restart openshift-master* to start the master services on the OSE-master.
* are all OSE worker-nodes are running? On each OSE-node execute *systemctl status openshift-node* and, to restart the services, use *systemctl restart openshift-node*, done on each OSE-node.
* is the OSE Web Console accessible? Login to the OpenShift Console via the GUI at https://ose-master-host-name-or-ip:8443/console
* is the *oc* command available? On the OSE-master server login to OSE via the command line using *oc login -u admin*:
```
$ oc login -u admin
Password:
Using project "default"

$ oc get projects
NAME              DISPLAY NAME   STATUS
default                          Active
openshift                        Active
openshift-infra                  Active
```

### Ceph on each OSE-node:
Each schedulable OSE-node needs the ceph-common library installed, and for now, due to a current ceph packaging bug, also needs full ceph installed.

Note: in order to install the full ceph, each OSE node may need certain ceph repos enabled.

```
$ subscription-manager repos --enable=rhel-7-server-rpms --enable=rhel-7-server-rhceph-1.3-installer-rpms \
      --enable=rhel-7-server-rhceph-1.3-mon-rpms --enable=rhel-7-server-rhceph-1.3-osd-rpms
```

Now install ceph on schedulable OSE-node:

```
$ yum install -y ceph-common

#and due to the ceph packaging bug where ceph-rbdnamed() is missing
$ yum install -y ceph
```

### Ceph Secret:
The ceph-rbd storage plugin uses a ceph secret for authorization. This is a short yaml file which resides on the OSE-master host but gets its value from the ceph monitor host.

```
#on a ceph monitor server:
$ ceph auth get-key client.admin
AQDva7JVEuVJBBAAc8e1ZBWhqUB9K/zNZdOHoQ==

$ echo "AQDva7JVEuVJBBAAc8e1ZBWhqUB9K/zNZdOHoQ=="| base64
QVFEdmE3SlZFdVZKQkJBQWM4ZTFaQldocVVCOUsvek5aZE9Ib1E9PQo=
# copy the above output
```

Back on the OSE-master node, edit the [ceph-secret file](ceph-secret.yaml) pasting the base64 value above. Then *oc create* the ceph-secret object:

```
$ oc create -f ceph-secret.yaml 
secrets/ceph-secret
 
$ oc get secret
NAME                  TYPE                                  DATA
ceph-secret           Opaque                                1
...
```

### Security:
OSE Security Context Constraints (SCC) are described in this [OSE Authorization Guide](https://docs.openshift.com/enterprise/3.0/architecture/additional_concepts/authorization.html#security-context-constraints). The *privileged* and *restricted* SCCs are added as defaults by OSE and need to be modified in order for mysql and ceph to have sufficient privileges. See also [General OSE Authorization](https://docs.openshift.com/enterprise/3.0/architecture/additional_concepts/authorization.html).

After logging-in to OSE as the *admin* user, edit the two SCC files, as shown below:

```
$ oc login -u admin
$ oc edit scc privilege
$ oc edit scc restricted
#change "MustRunAsRange" to "RunAsAny"

$ oc get scc
NAME         PRIV      CAPS      HOSTDIR   SELINUX    RUNASUSER
privileged   true      []        true      RunAsAny   RunAsAny
restricted   false     []        false     RunAsAny   RunAsAny
```

### MySQL:
For each OSE-node see the [setting up mysql doc](MYSQL.md).

### Verification/Validation:

On the OSE master host:

```
$ systemctl status openshift-master -l
openshift-master.service - OpenShift Master
   Loaded: loaded (/usr/lib/systemd/system/openshift-master.service; enabled)
   Active: active (running) since Thu 2015-09-03 16:35:56 EDT; 8h ago
     Docs: https://github.com/openshift/origin
 Main PID: 49702 (openshift)
   CGroup: /system.slice/openshift-master.service
           └─49702 /usr/bin/openshift start master --config=/etc/openshift/master/master-config.yaml --loglevel=4
...
DeploymentConfigs for trigger on ImageStream openshift/mysql
Sep 04 00:46:02 rhel7-ose-1 openshift-master[49702]: I0904 00:46:02.803183   49702 controller.go:38] Detecting changed images for DeploymentConfig default/docker-registry:2
...
Sep 04 00:46:03 rhel7-ose-1 openshift-master[49702]: I0904 00:46:03.001061   49702 controller.go:85] Ignoring DeploymentConfig change for default/docker-registry:2 (latestVersion=2); same as Deployment default/docker-registry-2
```

And, still on the OSE-master:

```
$ oc get nodes
NAME              LABELS                                   STATUS
192.168.122.179   kubernetes.io/hostname=192.168.122.179   Ready,SchedulingDisabled
192.168.122.254   kubernetes.io/hostname=192.168.122.254   Ready

```

On *each* OSE schedulable node:

```
$ rpm -qa|grep ceph
ceph-common-0.94.1-16.el7cp.x86_64
ceph-0.94.1-16.el7cp.x86_64
```

```
$ systemctl status openshift-node -l
openshift-node.service - OpenShift Node
   Loaded: loaded (/usr/lib/systemd/system/openshift-node.service; enabled)
  Drop-In: /usr/lib/systemd/system/openshift-node.service.d
           └─openshift-sdn-ovs.conf
   Active: active (running) since Tue 2015-09-01 18:58:27 EDT; 2 days ago
     Docs: https://github.com/openshift/origin
 Main PID: 94526 (openshift)
   CGroup: /system.slice/openshift-node.service
           └─94526 /usr/bin/openshift start node --config=/etc/openshift/node/node-config.yaml --loglevel=4


Sep 04 00:48:09 rhel7-ose-2 openshift-node[94526]: I0904 00:48:09.198143   94526 manager.go:1388] Pod infra container looks good, keep it "mysql_default"
Sep 04 00:48:09 rhel7-ose-2 openshift-node[94526]: I0904 00:48:09.198195   94526 manager.go:1411] pod "mysql_default" container "mysql" exists as 77f4af567e3dd3b10656ad5ee38a39600174a87c519d6a23735e96cf0ee4208a
Sep 04 00:48:09 rhel7-ose-2 openshift-node[94526]: I0904 00:48:09.198214   94526 prober.go:180] Readiness probe for "mysql_default:mysql" succeeded
Sep 04 00:48:09 rhel7-ose-2 openshift-node[94526]: I0904 00:48:09.198224   94526 manager.go:1442] probe success: "mysql"
Sep 04 00:48:09 rhel7-ose-2 openshift-node[94526]: I0904 00:48:09.198239   94526 manager.go:1515] Got container changes for pod "mysql_default": {StartInfraContainer:false InfraContainerId:dca749fa3530d552a643a836051d4b00ef4e3a69c69ebc8ede059848b3b27569 ContainersToStart:map[] ContainersToKeep:map[dca749fa3530d552a643a836051d4b00ef4e3a69c69ebc8ede059848b3b27569:-1 77f4af567e3dd3b10656ad5ee38a39600174a87c519d6a23735e96cf0ee4208a:0]}
Sep 04 00:48:09 rhel7-ose-2 openshift-node[94526]: I0904 00:48:09.198268   94526 kubelet.go:2245] Generating status for "mysql_default"
...
Sep 04 00:48:09 rhel7-ose-2 openshift-node[94526]: I0904 00:48:09.202366   94526 status_manager.go:129] Ignoring same status for pod "mysql_default", status: {Phase:Running Conditions:[{Type:Ready Status:True}] Message: Reason: HostIP:192.168.122.254 PodIP:10.1.0.41 StartTime:2015-09-03 19:10:07.598461546 -0400 EDT ContainerStatuses:[{Name:mysql State:{Waiting:<nil> Running:0xc213e6d0c0 Terminated:<nil>} LastTerminationState:{Waiting:<nil> Running:<nil> Terminated:<nil>} Ready:true RestartCount:0 Image:mysql ImageID:docker://7eee2d462c8f6ffacfb908cc930559e21778f60afdb2d7e9cf0f3025274d7ea8 ContainerID:docker://77f4af567e3dd3b10656ad5ee38a39600174a87c519d6a23735e96cf0ee4208a}]}
```

And some docker checks on *each* OSE node:

```
$ docker ps   #make sure docker is running

#if docker is not running then start it:
$ systemctl start docker
$ systemctl enable docker
$ systemctl status docker -l
```

### Log Files
*journalctl* and *systemctl status* are the main ways to view OSE log files. The *systemctl status* command is shown above. Here are some *journalctl* examples:

```
$ journalctl -xe -u openshift-master

#and on the OSE node:
$ journalctl -xe -u openshift-node
```

It's often necessary to scroll right (right arrow) to see pertinent info.
