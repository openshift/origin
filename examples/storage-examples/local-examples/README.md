# OpenShift Local Volume Examples  [WIP]

OpenShift allows for using local devices as PersistentVolumes.
This feature is alpha in 3.7 and must be explicitly enabled on all OpenShift
masters, controllers and nodes (see below).

## Alpha disclaimer

Local Volumes are alpha feature in 3.7. It requires several manual steps to
enable, configure and deplouy the feature. It may be reworked in the furute and
it will be probably automated by openshift-ansible.


## Overview
Local volumes are PersistentVolumes representing local mounted filesystems.
In the future they may be extended to raw block devices.

The main difference between HostPath and Local volume is that Local
PersistentVolumes have special annotation that makes any Pod that uses the PV
to be scheduled on the same node where the local volume is mounted.

In addition, Local volume comes with a provisioner that automatically creates
PVs for locally mounted devices. This provisioner is currently very limited
and just scans pre-configured directories. It cannot dynamically provision
volumes, it may be implemented in a future release.

## Enabling Local Volumes

All OpenShift masters and nodes must run with enabled feature
`PersistentLocalVolumes=true`. Edit `master-config.yaml` on all master hosts and
make sure that `apiServerArguments` and `controllerArguments` enable the feature:

```yaml
apiServerArguments:
  feature-gates:
  - PersistentLocalVolumes=true
  ...

controllerArguments:
  feature-gates:
  - PersistentLocalVolumes=true
  ...
```

Similarly, the feature needs to be enabled on all nodes. Edit `node-config.yaml`
on all nodes:

```yaml
kubeletArguments:
  feature-gates:
  - PersistentLocalVolumes=true
  ...
```

## Mounting Local Volumes

While the feature is in alpha all local volumes must be manually mounted before
they can be consumed by Kubernetes as PersistentVolumes.

All volumes must be mounted into
`/mnt/local-storage/<storage-class-name>/<volume>`. It's up to the administrator
to create the local devices as needed (using any method such as disk partition,
LVM, ...), create suitable filesystems on them and mount them, either by a
script or `/etc/fstab` entries.

Example of `/etc/fstab`:
```
# device name   # mount point                  # FS    # options # extra
/dev/sdb1       /mount/local-storage/ssd/disk1 ext4     defaults 1 2
/dev/sdb2       /mount/local-storage/ssd/disk2 ext4     defaults 1 2
/dev/sdb3       /mount/local-storage/ssd/disk3 ext4     defaults 1 2
/dev/sdc1       /mount/local-storage/hdd/disk1 ext4     defaults 1 2
/dev/sdc2       /mount/local-storage/hdd/disk2 ext4     defaults 1 2
```

## Prerequisites

While not strictly required, it's desirable to create a standalone namespace
for local volume provisioner and its configuration:

```bash
oc new-project local-storage
```

## Local provisioner configuration

OpenShift depends on an external provisioner to create PersistentVolumes for
local devices and to clean them up when they're not needed so they can be used
again.

This external provisioner needs to be configured via an ConfigMap to know what
directory represents which StorageClass:

```yaml
kind: ConfigMap
metadata:
  name: local-volume-config
data:
    "local-ssd": | <1>
      {
        "hostDir": "/mnt/local-storage/ssd", <2>
        "mountDir": "/mnt/local-storage/ssd" <3>
      }
    "local-hdd": |
      {
        "hostDir": "/mnt/local-storage/hdd",
        "mountDir": "/mnt/local-storage/hdd"
      }
```
* <1> Name of the StorageClass.
* <2> Path to the directory on the host. It must be a subdirectory of `/mnt/local-storage`.
* <3> Path to the directory in the provisioner pod. The same directory structure
  as on the host is strongly suggested.

With this configuration the provisioner will create:
* One PersistentVolume with StorageClass `local-ssd` for every subdirectory in `/mnt/local-storage/ssd`.
* One PersistentVolume with StorageClass `local-hdd` for every subdirectory in `/mnt/local-storage/hdd`.

This configuration must be created before the provisioner is deployed by the
template below!

## Local provisioner deployment

Note that all local devices must be mounted and ConfigMap with storage classes
and their respective directories must be created before starting the
provisioner!

The provisioner is installed from OpenShift template that's available at https://raw.githubusercontent.com/jsafrane/origin/local-storage/examples/storage-examples/local-examples/local-storage-provisioner-template.yaml.

1. Prepare a service account that will be able to run pods as root user, use
   HostPath volumes and run with any SELinux context:
   ```bash
   oc create serviceaccount local-storage-admin
   oc adm policy add-scc-to-user privileged -z local-storage-admin
   ```
   Root privileges and any SELinux context are necessary for the provisioner
   pod so it can delete any content on the local volumes. HostPath is necessary
   to access `/mnt/local-storage` on the host.

2. Install the template:
   ```bash
   oc create -f https://raw.githubusercontent.com/jsafrane/origin/local-storage/examples/storage-examples/local-examples/local-storage-provisioner-template.yaml
   ```
3. Instantiate the template. Specify value of "configmap" and "account"
   parameters:
   ```bash
   oc new-app -p CONFIGMAP=local-volume-config -p SERVICE_ACCOUNT=local-storage-admin -p NAMESPACE=local-storage local-storage-provisioner
   ```
   See the template for other configurable options.
   The template creates a DaemonSet that runs a Pod on every node. The Pod
   watches directories specified in the ConfigMap and creates PersistentVolumes
   for them automatically.

   Note that the provisioner runs as root to be able to clean up the directories
   when respective PersistentVolume is released and all data need to be removed.

## Adding new devices

Adding a new device requires several manual steps:

1. Stop DaemonSet with the provisioner.
2. Create a subdirectory in the right directory on the node with the new device
   and mount it there.
3. Start the DaemonSet with the provisioner.

Omitting any of these steps may result in a wrong PV being created!
