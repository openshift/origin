% OCI(1) OCI-RUNTIME-TOOL User Manuals
% OCI Community
% APRIL 2016
# NAME
oci-runtime-tool-generate - Generate a config.json for an OCI container

# SYNOPSIS
**oci-runtime-tool generate** *[OPTIONS]*

# DESCRIPTION

`oci-runtime-tool generate` generates configuration JSON for an OCI bundle.
By default, it writes the JSON to stdout, but you can use **--output**
to direct it to a file.  OCI-compatible runtimes like runC expect to
read the configuration from `config.json`.

# OPTIONS
**--args**=OPTION
  Arguments to run within the container.  Can be specified multiple times.
  If you were going to run a command with multiple options, you would need
  to specify the command and each argument in order.

  --args "/usr/bin/httpd" --args "-D" --args "FOREGROUND"

**--env**=[]
  Set environment variables e.g. key=value.
  This option allows you to specify arbitrary environment variables
  that are available for the process that will be launched inside of
  the container.

**--env-file**=[]
  Set environment variables from a file.
  This option sets environment variables in the container from the
  contents of a file formatted with key=value pairs, one per line.
  When specified multiple times, files are loaded in order with duplicate
  keys overwriting previous ones.

**--help**
  Print usage statement

**--hostname**=""
  Set the container host name that is available inside the container.

**--hooks-poststart-add**=[]
  Set command to run in poststart hooks. Can be specified multiple times.
  The multiple commands will be run in order before the container process
  gets launched but after the container environment and main process has been
  created.

  --hooks-poststart-add '{"path":"/bin/test","args":["a1","b2"],"timeout":5}'

  When same path specified over once, the last one make sense. So, you can
  take advantage of this to modify existing poststart hooks.

**--hooks-poststart-remove-all**=true|false
  Remove all poststart hooks. The default is *false*.
  When specifed with --hooks-poststart-add, will be applied first and then add
  new poststart hooks.

**--hooks-poststop-add**=[]
  Set command to run in poststop hooks. Can be specified multiple times.
  The multiple commands will be run in order after the container process
  is stopped.

  --hooks-poststop-add '{"path":"/bin/test","env":["a=b","c=d"],"timeout":5}'

  When same path specified over once, the last one make sense. So, you can
  take advantage of this to modify existing poststop hooks.

**--hooks-poststop-remove-all**=true|false
  Remove all poststop hooks. The default is *false*.
  When specifed with --hooks-postop-add, will be applied first and then add
  new poststop hooks.

**--hooks-prestart-add**=[]
  Set command to run in prestart hooks. Can be specified multiple times.
  The multiple commands will be run in order after the container process
  has been created but before it executes the user-configured code.

  --hooks-prestart-add '{"path":"/bin/test"}'

  When same path specified over once, the last one make sense. So, you can
  take advantage of this to modify existing prestart hooks.

**--hooks-prestart-remove-all**=true|false
  Remove all prestart hooks. The default is *false*.
  When specifed with --hooks-prestart-add, will be applied first and then add
  new prestart hooks.

**--label**=[]
  Add annotations to the configuration e.g. key=value.
  Currently, key containing equals sign is not supported.

**--linux-apparmor**=PROFILE
  Specifies the apparmor profile for the container

**--linux-blkio-leaf-weight**=WEIGHT
  Block IO (realtive leaf weight) accepts a weight value from 10 to 100.

**--linux-blkio-leaf-weight-device**=[]
  Block IO (realtive device leaf weight, format: MAJOR:MINOR:WEIGHT).
  This option can be specified multiple times. If a device was specified more than once, the last WEIGHT makes sense.
  The special *WEIGHT*  -1  removes existing setting for device MAJOR:MINOR.

**--linux-blkio-read-bps-device**=[]
  Limit read rate (bytes per second) from a device, format is MAJOR:MINOR:LIMIT
  e.g. --linux-blkio-read-bps-device=8:0:100
  This option can be specified multiple times. If a device was specified more than once, the last LIMIT makes sense.
  The special *LIMIT*  -1  removes existing setting for device MAJOR:MINOR.

**--linux-blkio-read-iops-device**=[]
  Limit read rate (IO per second) from a device, format is MAJOR:MINOR:LIMIT
  e.g. --linux-blkio-read-iops-device=8:0:1000
  This option can be specified multiple times. If a device was specified more than once, the last LIMIT makes sense.
  The special *LIMIT*  -1  removes existing setting for device MAJOR:MINOR.

**--linux-blkio-weight**=WEIGHT
  Block IO (realtive weight) accepts a weight value from 10 to 100.

**--linux-blkio-weight-device**=[]
  Block IO (realtive device weight, format: MAJOR:MINOR:WEIGHT).
  This option can be specified multiple times. If a device was specified more than once, the last WEIGHT makes sense.
  The special *WEIGHT*  -1  removes existing setting for device MAJOR:MINOR.

**--linux-blkio-write-bps-device**=[]
  Limit write rate (bytes per second) to a device, format is MAJOR:MINOR:LIMIT
  e.g. --linux-blkio-write-bps-device=8:0:100
  This option can be specified multiple times. If a device was specified more than once, the last LIMIT makes sense.
  The special *LIMIT*  -1  removes existing setting for device MAJOR:MINOR.

**--linux-blkio-write-iops-device**=[]
  Limit write rate (IO per second) to a device, format is MAJOR:MINOR:LIMIT
  e.g. --linux-blkio-write-iops-device=8:0:1000
  This option can be specified multiple times. If a device was specified more than once, the last LIMIT makes sense.
  The special *LIMIT*  -1  removes existing setting for device MAJOR:MINOR.
**--linux-cgroups-path**=""
  Specifies the path to the cgroups relative to the cgroups mount point.

**--linux-cpu-period**=CPUPERIOD
  Specifies a period of time in microseconds for how regularly a cgroup's access to CPU resources should be reallocated (CFS scheduler only).

**--linux-cpu-quota**=CPUQUOTA
  Specifies the total amount of time in microseconds for which all tasks in a cgroup can run during one period.

**--linux-cpus**=CPUS
  Sets the CPUs to use within the cpuset (default is to use any CPU available).

**--linux-cpu-shares**=CPUSHARES
  Specifies a relative share of CPU time available to the tasks in a cgroup.

**--linux-device-add**=*TYPE:MAJOR:MINOR:PATH[:OPTIONS...]*
  Add a device file in container. e.g. --device=c:10:229:/dev/fuse:fileMode=438:uid=0:gid=0
  The *TYPE*, *MAJOR*, *MINOR*, *PATH* are required.
    *TYPE* is the device type. The acceptable values are b (block), c (character), u (unbuffered), p (FIFO).
    *MAJOR*/*MINOR* is the major/minor device id.
    *PATH* is the device path.
  The *fileMode*, *uid*, *gid* are optional.
    *fileMode* is the file mode of the device file.
    *uid*/*gid* is the user/group id of the device file.
  This option can be specified multiple times.

**--linux-device-remove**=*PATH*
  Remove a device file in container.
  This option can be specified multiple times.

**--linux-device-remove-all**=true|false
  Remove all devices for linux inside the container. The default is *false*.
  This option conflicts with --linux-device-add and --linux-device-remove.
  When combined with them, no matter what the options' order is, parse this option first.

**--linux-device-cgroup-add**=allow|deny[,type=TYPE][,major=MAJOR][,minor=MINOR][,access=ACCESS]
  Add a device control rule.
  allow|deny: whether the entry is allowed or denied.
  TYPE: the device type. The value could be one of 'a' (all), 'b' (block), 'c' (character).
  MAJOR/MINOR: the major/minor id of device.
  ACCESS: cgroup permissions for device. A composition of r (read), w (write), and m (mknod).

**--linux-device-cgroup-remove**=allow|deny[,type=TYPE][,major=MAJOR][,minor=MINOR][,access=ACCESS]
  Remove a device control rule.
  The arguments is same as *--linux-device-cgroup-add*.
  
**--linux-disable-oom-kill**=true|false
  Whether to disable OOM Killer for the container or not.

**--linux-gidmappings**=GIDMAPPINGS
  Add GIDMappings e.g HostID:ContainerID:Size.  Implies **-user=**.

**--linux-hugepape-limits-add**=[]
  Add hugepage resource limits, format is PAGESIZE:LIMIT. e.g. --linux-hugepage-limits-add=4MB:102400
  This option can be specified multiple times. When same PAGESIZE specified over once, the last one make sense.

**--linux-hugepape-limits-drop**=[]
  Drop hugepage rsource limits. Just need to specify PAGESIZE. e.g. --linux-hugepage-limits-drop=4MB
  This option can be specified multiple times.

**--linux-intelRdt-l3CacheSchema**=""
  Specifies the schema for L3 cache id and capacity bitmask.

**--linux-masked-paths**=[]
  Specifies paths can not be read inside container. e.g. --linux-masked-paths=/proc/kcore
  This option can be specified multiple times.

**--linux-mem-kernel-limit**=MEMKERNELLIMIT
  Sets the hard limit of kernel memory in bytes.

**--linux-mem-kernel-tcp**=MEMKERNELTCP
  Sets the hard limit of kernel TCP buffer memory in bytes.

**--linux-mem-limit**=MEMLIMIT
  Sets the limit of memory usage in bytes.

**--linux-mem-reservation**=MEMRESERVATION
  Sets the soft limit of memory usage in bytes.

**--linux-mem-swap**=MEMSWAP
  Sets the total memory limit (memory + swap) in bytes.

**--linux-mem-swappiness**=MEMSWAPPINESS
  Sets the swappiness of how the kernel will swap memory pages (Range from 0 to 100).

**--linux-mems**=MEMS
  Sets the list of memory nodes in the cpuset (default is to use any available memory node).

**--linux-mount-label**=MOUNTLABEL
  Mount Label
  Depending on your SELinux policy, you would specify a label that looks like
  this:
  "system_u:object_r:svirt_sandbox_file_t:s0:c1,c2"

    Note you would want your ROOTFS directory to be labeled with a context that
    this process type can use.

      "system_u:object_r:usr_t:s0" might be a good label for a readonly container,
      "system_u:system_r:svirt_sandbox_file_t:s0:c1,c2" for a read/write container.

**--linux-namespace-add**=NSNAME[:PATH]
  Adds or replaces the given linux namespace NSNAME with a namespace entry that
  has a path of PATH. Omitting PATH means that a new namespace will be created
  by the container.

**--linux-namespace-remove**=NSNAME
  Removes a namespace from the set of namespaces configured in the container,
  so that the host's namespace will be used by the container instead of
  creating or joining another namespace.

**--linux-namespace-remove-all**=true|false
  Removes all namespaces from the set of namespaces configured for a container,
  such that the container will effectively run on the host.
  This option conflicts with --linux-namespace-add and --linux-namespace-remove.
  When combined with them, no matter what the options' order is, parse this option first.

**--linux-network-classid**=CLASSID
  Specifies network class identifier which will be tagged by container's network packets.

**--linux-network-priorities**=[]
  Specifies network priorities of network traffic, format is NAME:PRIORITY.
  e.g. --linux-network-priorities=eth0:123
  This option can be specified multiple times. If a interface name was specified more than once, the last PRIORITY makes sense.
  The special *PRIORITY*  -1  removes existing setting for interface NAME.

**--linux-oom-score-adj**=adj
  Specifies oom_score_adj for the container.

**--linux-pids-limit**=PIDSLIMIT
  Set maximum number of PIDs.

**--linux-readonly-paths**=[]
  Specifies paths readonly inside container. e.g. --linux-readonly-paths=/proc/sys
  This option can be specified multiple times.

**--linux-realtime-period**=REALTIMEPERIOD
  Sets the CPU period to be used for realtime scheduling (in usecs). Same as **--linux-cpu-period** but applies to realtime scheduler only.

**--linux-realtime-runtime**=REALTIMERUNTIME
  Specifies a period of time in microseconds for the longest continuous period in which the tasks in a cgroup have access to CPU resources.

**--linux-rootfs-propagation**=PROPAGATIONMODE
  Mount propagation for root filesystem.
  Values are "(r)shared, (r)private, (r)slave, (r)unbindable"

**--linux-seccomp-allow**=SYSCALL
  Specifies syscalls to be added to the ALLOW list.
  See --linux-seccomp-syscalls for setting limits on arguments.

**--linux-seccomp-arch**=ARCH
  Specifies Additional architectures permitted to be used for system calls.
  By default if you turn on seccomp, only the host architecture will be allowed.

**--linux-seccomp-default**=ACTION
  Specifies the the default action of Seccomp syscall restrictions and removes existing restrictions with the specified action
  Values: kill, trap, errno, trace, allow

**--linux-seccomp-default-force**=ACTION
  Specifies the the default action of Seccomp syscall restrictions
  Values: kill, trap, errno, trace, allow

**--linux-seccomp-errno**=SYSCALL
  Specifies syscalls to create seccomp rule to respond with ERRNO.

**--linux-seccomp-kill**=SYSCALL
  Specifies syscalls to create seccomp rule to respond with KILL.

**--linux-seccomp-only**=true|false
  Option to only export the seccomp section of output

**--linux-seccomp-remove**=[]
  Specifies syscall restrictions to remove from the configuration.

**--linux-seccomp-remove-all**=true|false
  Option to remove all syscall restrictions.
  This option conflicts with other --linux-seccomp-xxx options.
  When combined with them, no matter what the options' order is, parse this option first.

**--linux-seccomp-trace**=SYSCALL
  Specifies syscalls to create seccomp rule to respond with TRACE.

**--linux-seccomp-trap**=SYSCALL
  Specifies syscalls to create seccomp rule to respond with TRAP.

**--linux-selinux-label**=PROCESSLABEL
  SELinux Label
  Depending on your SELinux policy, you would specify a label that looks like
  this:
  "system_u:system_r:svirt_lxc_net_t:s0:c1,c2"

    Note you would want your ROOTFS directory to be labeled with a context that
    this process type can use.

      "system_u:object_r:usr_t:s0" might be a good label for a readonly container,
      "system_u:object_r:svirt_sandbox_file_t:s0:c1,c2" for a read/write container.

**--linux-sysctl**=SYSCTLSETTING
  Add sysctl settings e.g net.ipv4.forward=1, only allowed if the syctl is
  namespaced.

**--linux-uidmappings**=[]

  Add UIDMappings e.g HostUID:ContainerID:Size.  Implies **--user=**.

**--mounts-add**=[]
  Configures additional mounts inside container.
  This option can be specified multiple times.
  For example,
  A. Add tmpfs into container.
    --mounts-add '{"destination": "/tmp","type": "tmpfs","source": "tmpfs","options": ["nosuid","strictatime","mode=755","size=65536k"]}'
  B. Bind host directory into containeri.
    --mounts-add '{"destination": "/data","type": "bind","source": "/volumes/testing","options": ["rbind","rw"]}'
  C. mount for windows platform
    --mount-add '{"destination": "C:\\folder-inside-container","source": "C:\\folder-on-host","options": ["ro"]}'

**--mounts-remove**=[]
  Remove mounts to destination path from inside container.
  This option can be specified multiple times.

**--mounts-remove-all**=true|false
  Remove all mounts inside the container. The default is *false*.
  When specified with --mount-add, this option will be parsed first.

**--os**=OS
  Operating system used within the container.

**--output**=PATH
  Instead of writing the configuration JSON to stdout, write it to a
  file at *PATH* (overwriting the existing content if a file already
  exists at *PATH*).

**--privileged**=true|false
  Give extended privileges to this container. The default is *false*.

  By default, OCI containers are
“unprivileged” (=false) and cannot do some of the things a normal root process can do.

  When the operator executes **oci-runtime-tool generate --privileged**, OCI will enable access to all devices on the host as well as disable some of the confinement mechanisms like AppArmor, SELinux, and seccomp from blocking access to privileged processes.  This gives the container processes nearly all the same access to the host as processes generating outside of a container on the host.

**--process-cap-add-ambient**=[]
  Add Linux ambient capabilities

**--process-cap-add-bounding**=[]
  Add Linux bounding capabilities

**--process-cap-add-effective**=[]
  Add Linux effective capabilities

**--process-cap-add-inheritable**=[]
  Add Linux inheritable capabilities

**--process-cap-add-permitted**=[]
  Add Linux permitted capabilities

**--process-cap-drop-all**=true|false
  Drop all Linux capabilities
  This option conflicts with other cap options, as --process-cap-*.
  When combined with them, no matter what the options' order is, parse this option first.

**--process-cap-drop-ambient**=[]
  Drop Linux ambient capabilities

**--process-cap-drop-bounding**=[]
  Drop Linux bounding capabilities

**--process-cap-drop-effective**=[]
  Drop Linux effective capabilities

**--process-cap-drop-inheritable**=[]
  Drop Linux inheritable capabilities

**--process-cap-drop-permitted**=[]
  Drop Linux permitted capabilities

**--process-consolesize**=WIDTH:HEIGHT
  Specifies the console size in characters of the terminal. e.g. --process-consolesize=80:40

**--process-cwd**=PATH
  Current working directory for the process. The default is */*.

**--process-gid**=GID
  Gid for the process inside of container

**--process-groups**=GROUP
  Supplementary groups for the processes inside of container

**--process-no-new-privileges**=true|false
  Set no new privileges bit for the container process.  Setting this flag
  will block the container processes from gaining any additional privileges
  using tools like setuid apps.  It is a good idea to run unprivileged
  containers with this flag.

**--process-rlimits-add**=[]
  Specifies resource limits, format is RLIMIT:HARD:SOFT. e.g. --process-rlimits-add=RLIMIT_NOFILE:1024:1024
  This option can be specified multiple times. When same RLIMIT specified over once, the last one make sense.

**--process-rlimits-remove**=[]
  Remove the specified resource limits for process inside the container.
  This option can be specified multiple times.

**--process-rlimits-remove-all**=true|false
  Remove all resource limits for process inside the container. The default is *false*.
  This option conflicts with --linux-rlimits-add and --linux-rlimits-remove.
  When combined with them, no matter what the options' order is, parse this option first.

**--process-terminal**=true|false
  Specifies whether a terminal is attached to the process. The default is *false*.

**--process-uid**=UID
  Sets the UID used within the container.

**--process-username**=""
  Sets the username used within the container.

**--rootfs-path**=ROOTFSPATH
  Path to the rootfs, which can be an absolute path or relative to bundle path.
  e.g the absolute path of rootfs is /to/bundle/rootfs, bundle path is /to/bundle,
  then the value set as ROOTFSPATH should be `/to/bundle/rootfs` or `rootfs`. The default is *rootfs*.

**--rootfs-readonly**=true|false
  Mount the container's root filesystem as read only.

  By default a container will have its root filesystem writable allowing processes to write files anywhere.  By specifying the `--rootfs-readonly` flag the container will have its root filesystem mounted as read only prohibiting any writes.

**--solaris-anet**=[]
  Represents the automatic creation of a network resource for an application container
  e.g. --solaris-anet '{"allowedAddress": "172.17.0.2/16","configureAllowedAddress": "true","linkname": "net0"}'

**--solaris-capped-cpu-ncpus**=""
  Specifies the percentage of CPU usage.
  An ncpu value of 1 means 100% of a CPU, a value of 1.25 means 125%, .75 mean 75%, and so forth.

**--solaris-capped-memory-physical**=""
  Specifies the physical caps on the memory.

**--solaris-capped-memory-swap**=""
  Specifies the swap caps on the memory.

**--solaris-limitpriv**=""
  Sets privilege limit.

**--solaris-max-shm-memory**=""
  Sets the maximum amount of shared memory.

**--solaris-milestone**=""
  Sets the SMF FMRI.

**--template**=PATH
  Override the default template with your own.
  Additional options will only adjust the relevant portions of your template.
  Templates are not validated for correctness, so the user should ensure that they are correct.

**--windows-hyperv-utilityVMPath**=PATH
  Specifies the path to the image used for the utility VM.

**--windows-ignore-flushes-during-boot**=true|false
  Whether to ignore flushes during boot.

**--windows-layer-folders**=[]
  Specifies a list of layer folders the container image relies on.

**--windows-network**=""
  Specifies network for container.
  e.g. --windows-network='{"endpointList": ["7a010682-17e0-4455-a838-02e5d9655fe6"],"allowUnqualifiedDNSQuery": true,"DNSSearchList": ["a.com"],"networkSharedContainerName": "containerName"}'

**--windows-resources-cpu**=""
  Specifies cpu for container.
  e.g. --windows-resources-cpu '{"count":100, "maximum": 5000}'

**--windows-resources-memory-limit**=MEMORYLIMIT
  Set limit of memory.

**--windows-resources-storage**=""
  Specifies storage for container.
  e.g. --windows-resources-storage '{"iops": 50, "bps": 20}'

**--windows-servicing**=true|false
  Whether to servicing operations

# EXAMPLES

## Generating container in read-only mode

During container image development, containers often need to write to the image
content.  Installing packages into /usr, for example.  In production,
applications seldom need to write to the image.  Container applications write
to volumes if they need to write to file systems at all.  Applications can be
made more secure by generating them in read-only mode using the --rootfs-readonly switch.
This protects the containers image from modification. Read only containers may
still need to write temporary data.  The best way to handle this is to mount
tmpfs directories on /generate and /tmp.

    $ oci-runtime-tool generate --rootfs-readonly --mounts-add '{"destination": "/tmp","type": "tmpfs","source": "tmpfs","options": ["nosuid","strictatime","mode=755","size=65536k"]}' --mounts-add '{"destination": "/run","type": "tmpfs","source": "tmpfs","options": ["nosuid","strictatime","mode=755","size=65536k"]}' --rootfs-path /var/lib/containers/fedora --args bash

## Exposing log messages from the container to the host's log

If you want messages that are logged in your container to show up in the host's
syslog/journal then you should bind mount the /dev/log directory as follows.

    $ oci-runtime-tool generate --mounts-add '{"destination": "/dev/log","type": "bind","source": "/dev/log","options": ["rbind","rw"]}' --rootfs-path /var/lib/containers/fedora --args bash

From inside the container you can test this by sending a message to the log.

    (bash)# logger "Hello from my container"

Then exit and check the journal.

    # exit

    # journalctl -b | grep Hello

This should list the message sent to logger.

## Bind Mounting External Volumes

To mount a host directory as a container volume, specify the absolute path to
the directory and the absolute path for the container directory separated by a
colon:

    $ oci-runtime-tool generate --mounts-add '{"destination": "/var/db","type": "bind","source": "/data1","options": ["rbind","rw"]}' --rootfs-path /var/lib/containers/fedora --args bash

## Using SELinux

You can use SELinux to add security to the container.  You must specify the process label to run the init process inside of the container using `--linux-selinux-label`.

    $ oci-runtime-tool generate --mounts-add '{"destination": "/var/db","type": "bind","source": "/data1","options": ["rbind","rw"]}' --linux-selinux-label system_u:system_r:svirt_lxc_net_t:s0:c1,c2 --linux-mount-label system_u:object_r:svirt_sandbo x_file_t:s0:c1,c2 --rootfs-path /var/lib/containers/fedora --args bash

Not in the above example we used a type of svirt_lxc_net_t and an MCS Label of s0:c1,c2.  If you want to guarantee separation between containers, you need to make sure that each container gets launched with a different MCS Label pair.

Also the underlying rootfs must be labeled with a matching label.  For the example above, you would execute a command like:

    # chcon -R system_u:object_r:svirt_sandbox_file_t:s0:c1,c2  /var/lib/containers/fedora

This will set up the labeling of the rootfs so that the process launched would be able to write to the container.  If you wanted to only allow it to read/execute the content in rootfs, you could execute:

    # chcon -R system_u:object_r:usr_t:s0  /var/lib/containers/fedora

When using SELinux, be aware that the host has no knowledge of container SELinux
policy. Therefore, in the above example, if SELinux policy is enforced, the
`/var/db` directory is not writable to the container. A "Permission Denied"
message will occur and an avc: message in the host's syslog.

To work around this, the following command needs to be generate in order for the proper SELinux policy type label to be attached to the host directory:

    # chcon -Rt svirt_sandbox_file_t -l s0:c1,c2 /var/db

Now, writing to the /data1 volume in the container will be allowed and the
changes will also be reflected on the host in /var/db.

# SEE ALSO
**runc**(1), **oci-runtime-tool**(1)

# HISTORY
April 2016, Originally compiled by Dan Walsh (dwalsh at redhat dot com)
