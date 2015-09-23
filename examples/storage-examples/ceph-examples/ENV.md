OSE Environment
===============

* 2 RHEL-7 VMs for running the openshift enterprise (OSE) master and node hosts
* A working ceph cluster, which can be a bare metal cluster, one or more VMs, one or more containers, or a very simple all-in-one container.

The RHEL-7 hosts running the OSE master and OSE nodes should have the following services enabled and running:
* selinux (*setenforce 1*)
* iptables (*systemctl start iptables*)
* firewalld (*systemctl start firewalld*) Note, if you cannot start firewalld due to the service being masked, you can do a *systemctl unmask firewalld* and then restart it
* all OSE nodes (master and workers) and the ceph host need to be running docker. Currently docker version 1.8 has storage setup issues, so see below on how to upgrade or downgrade the version of docker on your VMs.

### Docker:
Docker may be on the OS running OSE, but make sure the docker version is 1.6 or 1.7 -- not 1.8.

```
$ docker --version
Docker version 1.6.0, build 350a636/1.6.0
```

#### Downgrading Docker:
There seems to be a docker 1.8 problem with storage setup where the docker-metapool is created too small. So, on docker version 1.8 consider downgrading to 1.7 or 1.6.

```
$ yum --showduplicates  list | grep ^docker

#if you see docker 1.6 or 1.7 then...
$ yum install -y docker-1.6  #or docker-1.7
```

If the above docker downgrade fails, reporting "Error: Nothing to do", then first attempt a yum clean:

```
$ yum clean all
# redo yum install from above...
```

If the downgrade still fails then the docker target version rpm can be downloaded directly from docker.com. As of the time of this writing this link worked:

```
https://get.docker.com/rpm/1.7.1/fedora-21/RPMS/x86_64/docker-engine-1.7.1-1.fc21.x86_64.rpm
```

#### Upgrading Docker:
If docker is lower than 1.6 then:

```
yum install -y docker-1.6  #or docker-1.7
```


### Other Installations:
1. [OSE installation](OSE.md)
2. [MySQL installation](MYSQL.md)
