% crio.conf(5) Open Container Initiative Daemon
% Aleksa Sarai
% OCTOBER 2016

# NAME
crio.conf - CRI-O configuration file

# DESCRIPTION
The CRI-O configuration file specifies all of the available command-line options
for the crio(8) program, but in a TOML format that can be more easily modified
and versioned.

# FORMAT
The [TOML format][toml] is used as the encoding of the configuration file.
Every option and subtable listed here is nested under a global "crio" table.
No bare options are used. The format of TOML can be simplified to:

    [table]
    option = value

    [table.subtable1]
    option = value

    [table.subtable2]
    option = value

## CRIO TABLE

The `crio` table supports the following options:


**root**=""
  CRIO root dir (default: "/var/lib/containers/storage")

**runroot**=""
  CRIO state dir (default: "/var/run/containers/storage")

**storage_driver**=""
  CRIO storage driver (default is "devicemapper")

**storage_option**=[]
  CRIO storage driver option list (no default)

## CRIO.API TABLE

**listen**=""
  Path to crio socket (default: "/var/run/crio.sock")

## CRIO.RUNTIME TABLE

**conmon**=""
  Path to the conmon executable (default: "/usr/local/libexec/crio/conmon")

**conmon_env**=[]
  Environment variable list for conmon process (default: ["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",])

**pids_limit**=""
  Maximum number of processes allowed in a container (default: 1024)

**runtime**=""
  OCI runtime path (default: "/usr/bin/runc")

**selinux**=*true*|*false*
  Enable selinux support (default: false)

**signature_policy**=""
  Path to the signature policy json file (default: "", to use the system-wide default)

**seccomp_profile**=""
  Path to the seccomp json profile to be used as the runtime's default (default: "/etc/crio/seccomp.json")

**apparmor_profile**=""
  Name of the apparmor profile to be used as the runtime's default (default: "crio-default")

## CRIO.IMAGE TABLE

**default_transport**
  A prefix to prepend to image names that can't be pulled as-is (default: "docker://")

**image_volumes**=""
  Image volume handling ('mkdir', 'bind' or 'ignore') (default: "mkdir")
  mkdir: A directory is created inside the container root filesystem for the volumes.
  bind: A directory is created inside container state directory and bind mounted into
  the container for the volumes.
  ignore: All volumes are just ignored and no action is taken.

**insecure_registries**=""
  Enable insecure registry  communication,  i.e.,  enable  un-encrypted
  and/or untrusted communication.

  List  of  insecure registries can contain an element with CIDR notation
  to specify a whole  subnet.  Insecure  registries  accept  HTTP  and/or
  accept HTTPS with certificates from unknown CAs.

  Enabling  --insecure-registry  is useful when running a local registry.
  However, because its use creates  security  vulnerabilities  it  should
  ONLY  be  enabled  for testing purposes.  For increased security, users
  should add their CA to their system's list of trusted  CAs  instead  of
  using --insecure-registry.

**pause_command**=""
  Path to the pause executable in the pause image (default: "/pause")

**pause_image**=""
  Image which contains the pause executable (default: "kubernetes/pause")

**registries**=""
  Comma separated list of registries that will be prepended when pulling
  unqualified images

## CRIO.NETWORK TABLE

**network_dir**=""
  Path to CNI configuration files (default: "/etc/cni/net.d/")

**plugin_dir**=""
  Path to CNI plugin binaries (default: "/opt/cni/bin/")

# SEE ALSO
crio(8)

# HISTORY
Oct 2016, Originally compiled by Aleksa Sarai <asarai@suse.de>
