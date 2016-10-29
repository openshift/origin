# Local Cluster Management

- [Overview](#overview)
- [Getting Started](#getting-started)
  - [Linux](#linux)
  - [MacOS with Docker for Mac](#macos-with-docker-for-mac)
  - [Mac OS X with Docker Toolbox](#mac-os-x-with-docker-toolbox)
  - [Windows with Docker for Windows](#windows-with-docker-for-windows)
  - [Windows with Docker Toolbox](#windows-with-docker-toolbox)
- [Installing Metrics](#installing-metrics)
- [Intalling Logging Aggregation](#installing-logging-aggregation)
- [Administrator Access](#administrator-access)
- [Docker Machine](#docker-machine)
- [Configuration](#configuration)
- [Etcd Data](#etcd-data)
- [Routing](#routing)
- [Specifying Images to Use](#specifying-images-to-use)

## Pre-requisites

| NOTE |
| ---- |
| This command was released with the 1.3+ version of oc client tools, so you must be using version 1.3+ or newer for this command to work. |


## Overview

The `oc cluster up` command starts a local OpenShift all-in-one cluster with a configured registry, router, image streams, and default templates.
By default, the command requires a working Docker connection. However, if running in an environment with 
[Docker Machine](https://docs.docker.com/machine) installed, it can create a Docker machine for you.

The `oc cluster up` command will create a default user and project and, once it completes, will allow you to start using the 
command line to create and deploy apps with commands like `oc new-app`, `oc new-build`, and `oc run`. It will also print out
a URL to access the management console for your cluster.

## Getting Started

### Linux

| WARNING |
| ------- |
| In some cases, networking for pods will not work for containers in your cluster, especially in Fedora 24. To fix this, flush your iptables rules by running `$ sudo iptables -F` before running `oc cluster up`. |

1. Install Docker with your platform's package manager.
2. Configure the Docker daemon with an insecure registry parameter of `172.30.0.0/16`
   - In RHEL and Fedora, edit the `/etc/sysconfig/docker` file and add or uncomment the following line:
     ```
     INSECURE_REGISTRY='--insecure-registry 172.30.0.0/16'
     ```

   - After editing the config, restart the Docker daemon.
     ```
     $ sudo systemctl restart docker
     ```

3. Download the Linux `oc` binary from
   [openshift-origin-client-tools-VERSION-linux-64bit.tar.gz](https://github.com/openshift/origin/releases)
   and place it in your path.

   > Please be aware that the 'oc cluster' set of commands are only available in the 1.3+ or newer releases.


4. Open a terminal with a user that has permission to run Docker commands and run:
   ```
   $ oc cluster up
   ```

To stop your cluster, run:
```
$ oc cluster down
```

### MacOS with Docker for Mac

1. Install [Docker for Mac](https://docs.docker.com/docker-for-mac/) making sure you meet the [prerequisites](https://docs.docker.com/docker-for-mac/#/what-to-know-before-you-install).
2. Once Docker is running, add an insecure registry of `172.30.0.0/16`:
   - From the Docker menu in the toolbar, select `Preferences...`
   - Click on `Advanced` in the preferences dialog
   - Under `Insecure registries:`, click on the `+` icon to add a new entry
   - Enter `172.30.0.0/16` and press `return`
   - Click on `Apply and Restart`
3. Install `socat`
   - If not already installed, install [Homebrew for Mac](http://brew.sh/)
   - Install socat
     Open Terminal and run:
     ```
     $ brew install socat
     ```
2. Download the Mac OS `oc` binary from [openshift-origin-client-tools-VERSION-mac.zip](https://github.com/openshift/origin/releases) and place it in your path.

   > Please be aware that the 'oc cluster' set of commands are only available in the 1.3+ or newer releases.

3. Open Terminal and run
   ```
   $ oc cluster up
   ```

To stop your cluster, run:
```
$ oc cluster down
```

### Mac OS X with Docker Toolbox

1. Install [Docker Toolbox](https://www.docker.com/products/docker-toolbox) and ensure that it is functional.
2. Download the OS X `oc` binary from [openshift-origin-client-tools-VERSION-mac.zip](https://github.com/openshift/origin/releases) and place it in your path.

   > Please be aware that the 'oc cluster' set of commands are only available in the 1.3+ or newer releases.

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

### Windows with Docker for Windows

1. Install [Docker for Windows](https://docs.docker.com/docker-for-windows/) making sure you meet the [prerequisites](https://docs.docker.com/docker-for-windows/#/what-to-know-before-you-install).
2. Once Docker is running, add an insecure registry of `172.30.0.0/16`:
   - Right click on the Docker icon in the notification area and select `Settings...`
   - Click on `Docker Daemon` in the settings dialog
   - Edit the Docker daemon configuration by adding `"172.30.0.0/16"` to the `"insecure-registries":` setting
     ```
     {
       "registry-mirrors": [],
       "insecure-registries": [ "172.30.0.0/16" ]
     }
     ```
   - Click on `Apply` and Docker will restart
3. Download the Windows `oc.exe` binary from [openshift-origin-client-tools-VERSION-windows.zip](https://github.com/openshift/origin/releases) and place it in your path.

   > Please be aware that the 'oc cluster' set of commands are only available in the 1.3+ or newer releases.

4. Open a Command window as Administrator and run:
   ```
   C:\> oc cluster up
   ```

To stop the cluster, run:

```
C:\> oc cluster down
```

### Windows with Docker Toolbox

1. Install [Docker Toolbox](https://www.docker.com/products/docker-toolbox) and ensure that it is functional.
2. Download the Windows `oc.exe` binary from [openshift-origin-client-tools-VERSION-windows.zip](https://github.com/openshift/origin/releases) and place it in your path.

   > Please be aware that the 'oc cluster' set of commands are only available in the 1.3+ or newer releases.

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

## Installing Metrics

You can install metrics components by specifying the --metrics argument when invoking `oc cluster up`.

To see metrics in the web console, you must first browse to the Hawkular metrics UI URL displayed when `cluster up` starts.


## Installing Logging Aggregation

| NOTE |
| ---- |
| This feature requires an oc command v1.4 or newer |

You can install logging aggregation components by specifying the --logging argument when invoking `oc cluster up`.

With logging aggregation installed, a new link will appear in the logs tab of a running pod in the web console.


## Administrator Access

To login as administrator to your cluster, login as `system:admin`:
```
oc login -u system:admin
```
Cluster administration commands are available under `oc adm`

To return to the regular `developer` user, login as that user:
```
oc login -u developer
```

## Docker Machine

By default, when `--create-machine` is used to create a new Docker machine, the `oc cluster up` command will use the
VirtualBox driver. In order to use a different driver, you must create the Docker machine beforehand
and either specify its name with the `--docker-machine` argument, or set its environment using the `docker-machine env`
command. When creating a Docker machine manually, you must specify the `--engine-insecure-registry` argument with the
value expected by OpenShift.

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
