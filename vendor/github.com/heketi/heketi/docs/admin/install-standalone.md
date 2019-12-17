
# Standalone Installation
Heketi can be installed on any system as long as it has access to all storage nodes that it will be required to manage.  Also, please keep in mind that the database today is a single point of failure, so it must be installed in a safe location (see [#208](https://github.com/heketi/heketi/issues/208)).

## EPEL/Fedora Installation
To install Heketi on an EPEL/Fedora please type the following:

* Fedora: 

```
# dnf install heketi
```

* EPEL

```
Server:
# yum install heketi
Client:
# yum install heketi-client
```



## Other Linux Systems
Download the latest Heketi version from [Releases](https://github.com/heketi/heketi/releases). For example:

```
$ wget https://github.com/heketi/heketi/releases/download/1.0.1/heketi-1.0.1.linux.amd64.tar.gz
$ tar xzvf heketi-1.0.1.linux.amd64.tar.gz
$ cd heketi
$ ./heketi -version
Heketi v1.0.1
```

## Installing as a container
You can also install Heketi as a container.  Please see https://hub.docker.com/r/heketi/heketi for more information
