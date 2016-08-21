# Local Cluster Management

- [Overview](#overview)
- [Getting Started](#getting-started)
  - [Mac OS X](#mac-os-x)
  - [Windows](#windows)
  - [Linux](#linux)
- [Administrator Access](#administrator-access)
- [Docker Machine](#docker-machine)
- [Configuration](#configuration)
- [Etcd Data](#etcd-data)
- [Routing](#routing)
- [Specifying Images to Use](#specifying-images-to-use)

## Overview

The `oc cluster up` command starts a local OpenShift all-in-one cluster with a configured registry, router, image streams, and default templates.
By default, the command requires a working Docker connection. However, if running in an environment with 
[Docker Machine](https://docs.docker.com/machine) installed, it can create a Docker machine for you.

The `oc cluster up` command will create a default user and project and, once it completes, will allow you to start using the 
command line to create and deploy apps with commands like `oc new-app`, `oc new-build`, and `oc run`. It will also print out
a URL to access the management console for your cluster.

## Getting Started

### Mac OS X

1. Install [Docker Toolbox](https://www.docker.com/products/docker-toolbox) and ensure that it is functional.
2. Download the OS X `oc` binary from [openshift-origin-client-tools-VERSION-mac.zip](https://github.com/openshift/origin/releases/latest) and place it in your path.
3. Open Terminal and run
   ```
   $ oc cluster up --create-machine
   ```

A Docker machine named `openshift` will be created using the VirtualBox driver and the OpenShift cluster 
will be started on it. 

To stop the cluster, run:

```
$ oc cluster down --docker-machine=openshift
```

To create a machine with a different name, specify the `--docker-machine` argument with `--create-machine`:

```
$ oc cluster up --create-machine --docker-machine=mymachine
```

Once the machine has been created, the `--create-machine` argument is no longer needed. To start/stop OpenShift again, either:

* Setup the Docker environment for the machine you wish to use, and then run `oc cluster up` and `oc cluster down`:

  ```
  $ eval $(docker-machine env openshift)
  $ oc cluster up

  ...

  $ oc cluster down
  ```

  OR

* Specify the Docker machine name as an argument to `oc cluster up` and `oc cluster down`:

  ```
  $ oc cluster up --docker-machine=openshift

  ...

  $ oc cluster down --docker-machine=openshift
  ```


### Windows

1. Install [Docker Toolbox](https://www.docker.com/products/docker-toolbox) and ensure that it is functional.
2. Download the Windows `oc.exe` binary from [openshift-origin-client-tools-VERSION-windows.zip](https://github.com/openshift/origin/releases/latest) and place it in your path.
3. Open a Command window as Administrator (for most drivers, docker-machine on Windows requires administrator privileges)
   and run:

   ```
   C:\> oc cluster up --create-machine
   ```

A Docker machine named `openshift` will be created using the VirtualBox driver and the OpenShift cluster 
will be started on it. 

To stop the cluster, run:

```
C:\> oc cluster down --docker-machine=openshift
```

To create a machine with a different name, specify the `--docker-machine` argument with `--create-machine`:

```
C:\> oc cluster up --create-machine --docker-machine=mymachine
```

Once the machine has been created, the `--create-machine` argument is no longer needed. To start/stop OpenShift again, either:

* Setup the Docker environment for the machine you wish to use, and then run `oc cluster up` and `oc cluster down`:
  ```
  C:\> @FOR /f "tokens=*" %i IN ('docker-machine env openshift') DO @%i
  C:\> oc cluster up

  ...

  C:\> oc cluster down
  ```

* Specify the Docker machine name as an argument to `oc cluster up` and `oc cluster down`:

  ```
  C:\> oc cluster up --docker-machine=openshift

  ...

  C:\> oc cluster down --docker-machine=openshift
  ```

### Linux

1. Install Docker with your platform's package manager. 
2. Configure the Docker daemon with an insecure registry parameter of `172.30.0.0/16`
   In RHEL and Fedora, edit the `/etc/sysconfig/docker` file and add or uncomment the following line:

   ```
   INSECURE_REGISTRY='--insecure-registry 172.30.0.0/16'
   ```
   After editing the config, restart the Docker daemon.

3. Download the Linux `oc` binary from 
   [openshift-origin-client-tools-VERSION-linux-64bit.tar.gz](https://github.com/openshift/origin/releases/latest) 
   and place it in your path.

4. Open a terminal with a user that has permission to run Docker commands and run:
   ```
   $ oc cluster up
   ```

To stop your cluster, run:
```
$ oc cluster down
```

## Administrator Access

To execute administrator commands on your cluster, `docker exec` into the `origin` container:

```
docker exec -ti origin bash
```

## Docker Machine

By default, when `--create-machine` is used to create a new Docker machine, the `oc cluster up` command will use the  
VirtualBox driver. In order to use a different driver, you must create the Docker machine beforehand
and either specify its name with the `--docker-machine` argument, or set its environment using the `docker-machine env`
command. When creating a Docker machine manually, you must specify the `--engine-insecure-registry` secure registry 
parameter expected by OpenShift.

Following are examples of creating a new Docker machine in OS X using the [xhyve](https://github.com/zchee/docker-machine-driver-xhyve) driver, 
and in Windows, using the [hyper-v](https://docs.docker.com/machine/drivers/hyper-v/) driver.

OS X:
```
$ docker-machine create --driver xhyve --engine-insecure-registry 172.30.0.0/16 mymachine
```

Windows (running a command window as Administrator):
```
C:\> docker-machine create --driver hyperv --engine-insecure-registry 172.30.0.0/16 mymachine
```

When the `--docker-machine` argument is specified on `oc cluster up`, the machine's environment does not need to be configured
on the current shell. Also if the machine exists but is not started, `oc cluster up` will attempt to start it.

## Configuration

`oc cluster up` creates its configuration by default in `/var/lib/origin/openshift.local.config` on the Docker host.
To specify a different location for it, use the `--host-config-dir` argument. The host directory will be mounted
in the `origin` container at `/var/lib/origin/openshift.local.config`.

A new configuration will be generated by default each time the cluster is started. To make changes to the configuration and 
preserve those changes, use the `--use-existing-config` argument when starting your cluster.

If your client is not the Docker host, you can make a local copy of the configuration with Docker cp:

```
docker cp origin:/var/lib/origin/openshift.local.config .
```

## Etcd Data

To persist data across restarts, specify a valid host directory in the `--host-data-dir` argument when starting your cluster
with `oc cluster up`. As long as the same value is specified every time, the data will be preserved across restarts.

If a host data directory is not specified, the data directory used by OpenShift is discarded when the container is destroyed.

## Routing

The default routing suffix used by `oc cluster up` is CLUSTER_IP.xip.io where CLUSTER_IP is the IP address of your cluster.
To use a different suffix, specify it with `--routing-suffix`.

## Specifying Images to Use

By default `oc cluster up` uses `openshift/origin:[released-version]` as its OpenShift image (where [released-version]
corresponds to the release of the `oc` client) and `openshift-origin-${component}:[released-version]` for
other images created by the OpenShift cluster (registry, router, builders, etc). It is possible to use a different set of 
images by specifying the version and/or the image prefix.

To use a different version of Origin, specify the --version argument. In the following example, images named 
openshift/origin:v1.1.6, openshift/origin-router:v1.1.6, etc. will be used for your cluster.
```
oc cluster up --version=v1.1.6
```

To use images from a different registry or with a different namespace, use the --image argument.  In the following example,
myregistry.example.com/ose/origin:latest, myregistry.example.com/ose/origin-router:latest, etc. will be used for your cluster.
```
oc cluster up --image=myregistry.example.com/ose/origin
```

Both --version and --image may be combined to specify the image name prefix and tag for the images to use.
