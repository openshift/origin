% storage.conf(5) Container Storage Configuration File
% Dan Walsh
% May 2017

# NAME
storage.conf - Syntax of Container Storage configuration file

# DESCRIPTION
The STORAGE configuration file specifies all of the available container storage options
for tools using shared container storage, but in a TOML format that can be more easily modified
and versioned.

# FORMAT
The [TOML format][toml] is used as the encoding of the configuration file.
Every option and subtable listed here is nested under a global "storage" table.
No bare options are used. The format of TOML can be simplified to:

    [table]
    option = value

    [table.subtable1]
    option = value

    [table.subtable2]
    option = value

## STORAGE TABLE

The `storage` table supports the following options:

**graphroot**=""
  container storage graph dir (default: "/var/lib/containers/storage")
  Default directory to store all writable content created by container storage programs

**runroot**=""
  container storage run dir (default: "/var/run/containers/storage")
  Default directory to store all temporary writable content created by container storage programs

**driver**=""
  container storage driver (default: "overlay")
  Default Copy On Write (COW) container storage driver

### STORAGE OPTIONS TABLE 

The `storage.options` table supports the following options:

**additionalimagestores**=[]
  Paths to additional container image stores. Usually these are read/only and stored on remote network shares.

**size**=""
  Maximum size of a container image.   This flag can be used to set quota on the size of container images. (default: 10GB)

**override_kernel_check**=""
  Tell storage drivers to ignore kernel version checks.  Some storage drivers assume that if a kernel is too
  old, the driver is not supported.  But for kernels that have had the drivers backported, this flag
  allows users to override the checks

**mount_program**=""
  Specifies the path to a custom program to use instead for mounting the file system.

**mountopt**=""

  Comma separated list of default options to be used to mount container images.  Suggested value "nodev".

[storage.options.thinpool]

Storage Options for thinpool

The `storage.options.thinpool` table supports the following options:

**autoextend_percent**=""

Tells the thinpool driver the amount by which the thinpool needs to be grown. This is specified in terms of % of pool size. So a value of 20 means that when threshold is hit, pool will be grown by 20% of existing pool size. (default: 20%)

**autoextend_threshold**=""

Tells the driver the thinpool extension threshold in terms of percentage of pool size. For example, if threshold is 60, that means when pool is 60% full, threshold has been hit. (default: 80%)

**basesize**=""

Specifies the size to use when creating the base device, which limits the size of images and containers. (default: 10g)

**blocksize**=""

Specifies a custom blocksize to use for the thin pool. (default: 64k)

**directlvm_device**=""

Specifies a custom block storage device to use for the thin pool. Required for using graphdriver `devicemapper`.

**directlvm_device_force**=""

Tells driver to wipe device (directlvm_device) even if device already has a filesystem.  (default: false)

**fs**="xfs"

Specifies the filesystem type to use for the base device. (default: xfs)

**log_level**=""

Sets the log level of devicemapper.

    0: LogLevelSuppress 0 (default)
    2: LogLevelFatal
    3: LogLevelErr
    4: LogLevelWarn
    5: LogLevelNotice
    6: LogLevelInfo
    7: LogLevelDebug

**min_free_space**=""

Specifies the min free space percent in a thin pool required for new device creation to succeed. Valid values are from 0% - 99%. Value 0% disables. (default: 10%)

**mkfsarg**=""

Specifies extra mkfs arguments to be used when creating the base device.

**use_deferred_removal**=""

Marks devicemapper block device for deferred removal.  If the device is in use when its driver attempts to remove it, the driver tells the kernel to remove the device as soon as possible.  Note this does not free up the disk space, use deferred deletion to fully remove the thinpool.  (default: true).

**use_deferred_deletion**=""

Marks thinpool device for deferred deletion. If the thinpool is in use when the driver attempts to delete it, the driver will attempt to delete device every 30 seconds until successful, or when it restarts.  Deferred deletion permanently deletes the device and all data stored in the device will be lost. (default: true).

**xfs_nospace_max_retries**=""

Specifies the maximum number of retries XFS should attempt to complete IO when ENOSPC (no space) error is returned by underlying storage device. (default: 0, which means to try continuously.)

**ostree_repo=""**
  Tell storage drivers to use the specified OSTree repository.  Some storage drivers, such as overlay, might use

**skip_mount_home=""**
  Tell storage drivers to not create a PRIVATE bind mount on their home directory.

# HISTORY
May 2017, Originally compiled by Dan Walsh <dwalsh@redhat.com>
Format copied from crio.conf man page created by Aleksa Sarai <asarai@suse.de>
