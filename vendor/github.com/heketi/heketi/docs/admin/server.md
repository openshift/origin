# Overview
Before starting the service for the first time, you need to setup the configuration file accordingly.  On EPEL/Fedora systems, the file is located in `/etc/heketi/heketi.json`.  On other Linux systems, the example configuration file comes as part of the tar file.

# Config file
The config file has information required to run the Heketi server.  The config file must be in JSON format with the following settings:

* port: _string_, Heketi REST service port number
* use_auth: _bool_, Enable JWT Authentication. For details,
  refer to the [API Authentication Model](../api/api.md#authentication-model)
* jwt: _map_, JWT Authentication settings
    * admin: _map_, Settings for the Heketi administrator
        * key: _string_, Shared secret
    * user: _map_, Settings for the Heketi volume requests access user
        * key: _string_, Shared secret
* glusterfs: _map_, GlusterFS settings
    * loglevel: _string_, Set log level.  Possible values are:
        * none, critical, error, warning, info, debug
    * executor: _string_, Determines the type of command executor to use.  Environment variable HEKETI_EXECUTOR can also be used to customize executor type.  Possible values are:
        * **mock**: Does not send any commands out to servers. Can be used for development and tests
        * **ssh**: Sends commands to real systems over ssh
        * **kubernetes**: Communicate with GlusterFS containers over Kubernetes exec
    * db: _string_, Location of Heketi database.  Environment variable HEKETI_DB_PATH can also be used to customize database location.
    * sshexec: _map_, SSH configuration
        * keyfile: _string_, File with private ssh key
        * user: _string_, SSH user
        * port: _string_, SSH port number
        * fstab: _string_, Fstab file where to store mount points
        * backup_lvm_metadata: _bool_, Create archives of the LVM metadata when running vgcreate/lvcreate
        * sudo: _bool_, set to true when SSHing as a non root user
	* debug_umount_failures: _bool_, Enable to capture more details in case brick unmounting fails. Can be overridden by the HEKETI_DEBUG_UMOUNT_FAILURES environment variable.
    * kubexec: _map_, Kubernetes configuration
        * host: _string_, Kubernetes API host.  Example `https://myhost:8443`.  Can also be use using environment variable HEKETI_KUBE_APIHOST
        * cert: _string_, Certificate file to for HTTPS connection. Can also be use using environment variable HEKETI_KUBE_CERTFILE
        * insecure: _bool_, Insecure HTTPS access, only use during testing. Can also be use using environment variable HEKETI_KUBE_INSECURE to _y_.
        * user: _string_, OpenShift/Kubernetes user to access Kubernetes API server. Can also be use using environment variable HEKETI_KUBE_USER.
        * password: _string_, Password for _user_. Can also be use using environment variable HEKETI_KUBE_PASSWORD.
        * namespace: _string_, Kubernetes namespace or OpenShift project where GlusterFS containers/Pods are running. Can also be use using environment variable HEKETI_KUBE_NAMESPACE.
        * fstab: _string_, Fstab file where to store mount points
        * backup_lvm_metadata: _bool_, Create archives of the LVM metadata when running vgcreate/lvcreate
	* debug_umount_failures: _bool_, Enable to capture more details in case brick unmounting fails. Can be overridden by the HEKETI_DEBUG_UMOUNT_FAILURES environment variable.

## Advanced Options
The following configuration options should only be set on advanced configurations under `glusterfs` section:
* brick_max_size_gb: _int_, Maximum brick size (Gb)
* brick_min_size_gb: _int_, Minimum brick size (Gb)
* max_bricks_per_volume: _int_, Maximum number of bricks per volume

Example:

```
...
	"glusterfs" : {
                ...
		"db" : "/var/lib/heketi/heketi.db",
		"brick_max_size_gb" : 1024,
		"brick_min_size_gb" : 1,
		"max_bricks_per_volume" : 33
                ...
	}
...
```

## Notes
* On EPEL/Fedora systems, the private ssh keyfile must be readable by `heketi` user.    

## Examples
* [Mock setup](https://github.com/heketi/heketi/blob/master/etc/heketi.json)
* ~~[Functional test setup](https://github.com/heketi/heketi/blob/master/tests/functional/large/config/heketi.json)~~

# Starting the server

## EPEL7/Fedora
To start Heketi, simply type:

```
# systemctl enable heketi
# systemctl start heketi
# systemctl status heketi
```

To see the logs, type:

```
journalctl -u heketi
```

The database will be installed in `/var/lib/heketi`.

## EPEL6
To start Heketi, type:

```
# chkconfig --add heketi
# service heketi start
```

To see the logs, type:

```
# less /var/log/heketi/heketi
```

The database will be installed in `/var/lib/heketi`.

## Other Linux Systems
To run the server you run the following command:

```
$ heketi --config=<config file>
```

The server will continue running in the foreground with all logs printed to standard output.

# Testing the configuration
To test that the server is running, please type the following:

* Using `curl` can be used if Heketi has not been setup with authentication:

```
$ curl http://<server:port>/hello
```

* Using heketi-cli

```
$ heketi-cli --server http://<server:port> --user <user> --secret <secret> cluster list
```

# Next
Please see [Topology Setup](./topology.md)
