# How To Use OpenStack Cinder Persistent Volumes

The purpose of this guide is to create Persistent Volumes using [OpenStack Cinder](https://wiki.openstack.org/wiki/Cinder). It is part of [OpenShift persistent storage guide](../README.md), which explains how to use these Persistent Volumes as data storage for applications.

This guide assumes knowledge of OpenShift fundamentals and that you have a cluster up and running on OpenStack.

## Cinder Provisioning

We'll be creating Cinder volumes in our OpenStack installation and pre-formatting them with ext3 filesystem. This requires Cinder and Nova client tools installed on a OpenStack virtual machine (=instance in OpenStack terminology) and configured OpenStack environment variables there. Consult your OpenStack site admins for values of these environment variables.
```console
[root@vm1 ~] $ yum install python-cinderclient python-keystoneclient python-novaclient
[root@vm1 ~] $ export OS_AUTH_URL=<auth. url>
[root@vm1 ~] $ export OS_TENANT_ID=<tenant id>
[root@vm1 ~] $ export OS_USERNAME=<username>
[root@vm1 ~] $ export OS_PASSWORD=<password>
[root@vm1 ~] $ export OS_REGION_NAME=<region>
```

Create 1GB and 5GB Cinder volumes and remember their IDs.
```console
[root@vm1 ~] $ cinder create --display-name test1 1
+---------------------+--------------------------------------+
|       Property      |                Value                 |
+---------------------+--------------------------------------+
|     attachments     |                  []                  |
|  availability_zone  |                 nova                 |
|       bootable      |                false                 |
|      created_at     |      2015-08-27T12:53:54.016972      |
| display_description |                 None                 |
|     display_name    |                test1                 |
|      encrypted      |                False                 |
|          id         | f37a03aa-6212-4c62-a805-9ce139fab180 |
|       metadata      |                  {}                  |
|         size        |                  1                   |
|     snapshot_id     |                 None                 |
|     source_volid    |                 None                 |
|        status       |               creating               |
|     volume_type     |                 None                 |
+---------------------+--------------------------------------+

[root@vm1 ~] $ cinder create --display-name test2 5
+---------------------+--------------------------------------+
|       Property      |                Value                 |
+---------------------+--------------------------------------+
|     attachments     |                  []                  |
|  availability_zone  |                 nova                 |
|       bootable      |                false                 |
|      created_at     |      2015-08-27T12:53:57.415840      |
| display_description |                 None                 |
|     display_name    |                test2                 |
|      encrypted      |                False                 |
|          id         | 51a3b34d-6f33-4e79-95f6-ebc804c96a1e |
|       metadata      |                  {}                  |
|         size        |                  5                   |
|     snapshot_id     |                 None                 |
|     source_volid    |                 None                 |
|        status       |               creating               |
|     volume_type     |                 None                 |
+---------------------+--------------------------------------+
```

Temporarily attach the volumes, format them with ext3 filesystem and change permissions of their root directory to allow anyone to write there. Both MySQL and WordPress will use non-root users to write to the volumes. Of course, use real VM instance ID instead of `<instance ID>` and real IDs of your volumes.

```console
[root@vm1 ~] $ nova volume-attach <instance ID> f37a03aa-6212-4c62-a805-9ce139fab180
+----------+--------------------------------------+
| Property | Value                                |
+----------+--------------------------------------+
| device   | /dev/vdd                             |
| id       | f37a03aa-6212-4c62-a805-9ce139fab180 |
| serverId | 338db252-2bc6-4de2-8941-b22faca3f3dd |
| volumeId | f37a03aa-6212-4c62-a805-9ce139fab180 |
+----------+--------------------------------------+

[root@vm1 ~] $ mkfs.ext3 /dev/vdd
mke2fs 1.42.11 (09-Jul-2014)
Creating filesystem with 262144 4k blocks and 65536 inodes
Filesystem UUID: 76a0669a-36e3-40e3-a4f7-ac5e207620c5
Superblock backups stored on blocks:
        32768, 98304, 163840, 229376

Allocating group tables: done
Writing inode tables: done
Creating journal (8192 blocks): done
Writing superblocks and filesystem accounting information: done

[root@vm1 ~] $ mount /dev/vdd /mnt
[root@vm1 ~] $ chmod 777 /mnt
[root@vm1 ~] $ umount /mnt
[root@vm1 ~] $ nova volume-detach <instance ID> f37a03aa-6212-4c62-a805-9ce139fab180

[root@vm1 ~] $ nova volume-attach <instance ID> 51a3b34d-6f33-4e79-95f6-ebc804c96a1e
+----------+--------------------------------------+
| Property | Value                                |
+----------+--------------------------------------+
| device   | /dev/vde                             |
| id       | 51a3b34d-6f33-4e79-95f6-ebc804c96a1e |
| serverId | 338db252-2bc6-4de2-8941-b22faca3f3dd |
| volumeId | 51a3b34d-6f33-4e79-95f6-ebc804c96a1e |
+----------+--------------------------------------+

[root@vm1 ~] $ mkfs.ext3 /dev/vde
mke2fs 1.42.11 (09-Jul-2014)
Creating filesystem with 1310720 4k blocks and 327680 inodes
Filesystem UUID: 47d983e7-17a6-4189-8a08-2edbad057555
Superblock backups stored on blocks:
        32768, 98304, 163840, 229376, 294912, 819200, 884736

Allocating group tables: done
Writing inode tables: done
Creating journal (32768 blocks): done
Writing superblocks and filesystem accounting information: ^[[A^[[Adone

[root@vm1 ~] $ mount /dev/vde /mnt
[root@vm1 ~] $ chmod 777 /mnt
[root@vm1 ~] $ umount /mnt
[root@vm1 ~] $ nova volume-detach <instance ID> 51a3b34d-6f33-4e79-95f6-ebc804c96a1e
```

These steps can be easily automated.  Scripting is left as an exercise for the reader.


## Cinder Persistent Volumes

Each Cinder volume becomes its own Persistent Volume in the cluster.

```console
# Edit Cinder persistent volume definitions and substitute <volume ID> with real ID of the volumes
[root@vm1 ~] $ vi examples/volumes//cinder/pv-cinder-1.yaml
    volumeID:  f37a03aa-6212-4c62-a805-9ce139fab180
[root@vm1 ~] $ vi examples/volumes//cinder/pv-cinder-2.yaml
    volumeID:  51a3b34d-6f33-4e79-95f6-ebc804c96a1e

[root@vm1 ~] $ oc create -f examples/volumes/cinder/pv-1.yaml
[root@vm1 ~] $ oc create -f examples/volumes/cinder/pv-2.yaml
[root@vm1 ~] $ oc get pv

NAME      LABELS    CAPACITY     ACCESSMODES   STATUS      CLAIM     REASON
pv0001    <none>    1073741824   RWO,RWX       Available             
pv0002    <none>    5368709120   RWO           Available             
```

Now the volumes are ready to be used by applications in the cluster.
