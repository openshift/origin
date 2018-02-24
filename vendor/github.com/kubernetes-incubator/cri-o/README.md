![CRI-O logo](https://cdn.rawgit.com/kubernetes-incubator/cri-o/master/logo/crio-logo.svg)
# CRI-O - OCI-based implementation of Kubernetes Container Runtime Interface

[![Build Status](https://img.shields.io/travis/kubernetes-incubator/cri-o.svg?maxAge=2592000&style=flat-square)](https://travis-ci.org/kubernetes-incubator/cri-o)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-incubator/cri-o?style=flat-square)](https://goreportcard.com/report/github.com/kubernetes-incubator/cri-o)

### Status: Stable

## Compatibility matrix: CRI-O <-> Kubernetes clusters

| Version - Branch           | Kubernetes branch/version     | Maintenance status |
|----------------------------|-------------------------------|--------------------|
| CRI-O 1.0.x  - release-1.0 | Kubernetes 1.7 branch, v1.7.x | =                  |
| CRI-O 1.8.x  - release-1.8 | Kubernetes 1.8 branch, v1.8.x | =                  |
| CRI-O 1.9.x  - release-1.9 | Kubernetes 1.9 branch, v1.9.x | =                  |
| CRI-O HEAD - master        | Kubernetes master branch      | ✓                  |

Key:

* `✓` Changes in main Kubernetes repo about CRI are actively implemented in CRI-O
* `=` Maintenance is manual, only bugs will be patched.

## What is the scope of this project?

CRI-O is meant to provide an integration path between OCI conformant runtimes and the kubelet.
Specifically, it implements the Kubelet [Container Runtime Interface (CRI)](https://github.com/kubernetes/community/blob/master/contributors/devel/container-runtime-interface.md) using OCI conformant runtimes.
The scope of CRI-O is tied to the scope of the CRI.

At a high level, we expect the scope of CRI-O to be restricted to the following functionalities:

* Support multiple image formats including the existing Docker image format
* Support for multiple means to download images including trust & image verification
* Container image management (managing image layers, overlay filesystems, etc)
* Container process lifecycle management
* Monitoring and logging required to satisfy the CRI
* Resource isolation as required by the CRI

## What is not in scope for this project?

* Building, signing and pushing images to various image storages
* A CLI utility for interacting with CRI-O. Any CLIs built as part of this project are only meant for testing this project and there will be no guarantees on the backward compatibility with it.

This is an implementation of the Kubernetes Container Runtime Interface (CRI) that will allow Kubernetes to directly launch and manage Open Container Initiative (OCI) containers.

The plan is to use OCI projects and best of breed libraries for different aspects:
- Runtime: [runc](https://github.com/opencontainers/runc) (or any OCI runtime-spec implementation) and [oci runtime tools](https://github.com/opencontainers/runtime-tools)
- Images: Image management using [containers/image](https://github.com/containers/image)
- Storage: Storage and management of image layers using [containers/storage](https://github.com/containers/storage)
- Networking: Networking support through use of [CNI](https://github.com/containernetworking/cni)

It is currently in active development in the Kubernetes community through the [design proposal](https://github.com/kubernetes/kubernetes/pull/26788).  Questions and issues should be raised in the Kubernetes [sig-node Slack channel](https://kubernetes.slack.com/archives/sig-node).

## Commands
| Command                                              | Description                                                               | Demo|
| ---------------------------------------------------- | --------------------------------------------------------------------------|-----|
| [crio(8)](/docs/crio.8.md)                           | OCI Kubernetes Container Runtime daemon                                   ||

Note that kpod and its container management and debugging commands have moved to a separate repository, located [here](https://github.com/projectatomic/libpod).

## Configuration
| File                                       | Description                                                                                          |
| ---------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| [crio.conf(5)](/docs/crio.conf.5.md)       | CRI-O Configuation file |

## OCI Hooks Support

[CRI-O configures OCI Hooks to run when launching a container](./hooks.md)

## CRI-O Usage Transfer

[Useful information for ops and dev transfer as it relates to infrastructure that utilizes CRI-O](/transfer.md)

## Communication

For async communication and long running discussions please use issues and pull requests on the github repo. This will be the best place to discuss design and implementation.

For sync communication we have an IRC channel #CRI-O, on chat.freenode.net, that everyone is welcome to join and chat about development.

## Getting started

### Runtime dependencies

- runc, Clear Containers runtime, or any other OCI compatible runtime
- socat
- iproute
- iptables

Latest version of `runc` is expected to be installed on the system. It is picked up as the default runtime by CRI-O.

### Build and Run Dependencies

**Required**

Fedora, CentOS, RHEL, and related distributions:

```bash
yum install -y \
  btrfs-progs-devel \
  device-mapper-devel \
  git \
  glib2-devel \
  glibc-devel \
  glibc-static \
  go \
  golang-github-cpuguy83-go-md2man \
  gpgme-devel \
  libassuan-devel \
  libgpg-error-devel \
  libseccomp-devel \
  libselinux-devel \
  ostree-devel \
  pkgconfig \
  runc \
  skopeo-containers
```

Debian, Ubuntu, and related distributions:

```bash
apt-get install -y \
  btrfs-tools \
  git \
  golang-go \
  libassuan-dev \
  libdevmapper-dev \
  libglib2.0-dev \
  libc6-dev \
  libgpgme11-dev \
  libgpg-error-dev \
  libseccomp-dev \
  libselinux1-dev \
  pkg-config \
  go-md2man \
  runc \
  skopeo-containers
```

Debian, Ubuntu, and related distributions will also need a copy of the development libraries for `ostree`, either in the form of the `libostree-dev` package from the [flatpak](https://launchpad.net/~alexlarsson/+archive/ubuntu/flatpak) PPA, or built [from source](https://github.com/ostreedev/ostree) (more on that [here](https://ostree.readthedocs.io/en/latest/#building)).

If using an older release or a long-term support release, be careful to double-check that the version of `runc` is new enough (running `runc --version` should produce `spec: 1.0.0`), or else build your own.

**NOTE**

Be careful to double-check that the version of golang is new enough, version 1.8.x or higher is required.  If needed, golang kits are avaliable at https://golang.org/dl/

**Optional**

Fedora, CentOS, RHEL, and related distributions:

(no optional packages)

Debian, Ubuntu, and related distributions:

```bash
apt-get install -y \
  libapparmor-dev
```

### Get Source Code

As with other Go projects, CRI-O must be cloned into a directory structure like:

```
GOPATH
└── src
    └── github.com
        └── kubernetes-incubator
            └── cri-o
```

First, configure a `GOPATH` (if you are using go1.8 or later, this defaults to `~/go`).

```bash
export GOPATH=~/go
mkdir -p $GOPATH
```

Next, clone the source code using:

```bash
mkdir -p $GOPATH/src/github.com/kubernetes-incubator
cd $_ # or cd $GOPATH/src/github.com/kubernetes-incubator
git clone https://github.com/kubernetes-incubator/cri-o # or your fork
cd cri-o
```

### Build

```bash
make install.tools
make
sudo make install
```

Otherwise, if you do not want to build `CRI-O` with seccomp support you can add `BUILDTAGS=""` when running make.

```bash
make BUILDTAGS=""
sudo make install
```

#### Build Tags

`CRI-O` supports optional build tags for compiling support of various features.
To add build tags to the make option the `BUILDTAGS` variable must be set.

```bash
make BUILDTAGS='seccomp apparmor'
```

| Build Tag | Feature                            | Dependency  |
|-----------|------------------------------------|-------------|
| seccomp   | syscall filtering                  | libseccomp  |
| selinux   | selinux process and mount labeling | libselinux  |
| apparmor  | apparmor profile support           | libapparmor |

### Running pods and containers

Follow this [tutorial](tutorial.md) to get started with CRI-O.

### Setup CNI networking

A proper description of setting up CNI networking is given in the
[`contrib/cni` README](contrib/cni/README.md). But the gist is that you need to
have some basic network configurations enabled and CNI plugins installed on
your system.

### Running with kubernetes

You can run a local version of kubernetes with CRI-O using `local-up-cluster.sh`:

1. Clone the [kubernetes repository](https://github.com/kubernetes/kubernetes)
1. Start the CRI-O daemon (`crio`)
1. From the kubernetes project directory, run:
```shell
CGROUP_DRIVER=systemd \
CONTAINER_RUNTIME=remote \
CONTAINER_RUNTIME_ENDPOINT='/var/run/crio/crio.sock  --runtime-request-timeout=15m' \
./hack/local-up-cluster.sh
```

To run a full cluster, see [the instructions](kubernetes.md).

### Current Roadmap

1. Basic pod/container lifecycle, basic image pull (done)
1. Support for tty handling and state management (done)
1. Basic integration with kubelet once client side changes are ready (done)
1. Support for log management, networking integration using CNI, pluggable image/storage management (done)
1. Support for exec/attach (done)
1. Target fully automated kubernetes testing without failures [e2e status](https://github.com/kubernetes-incubator/cri-o/issues/533)
1. Track upstream k8s releases
