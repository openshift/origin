# Etcd helper

A helper tool for getting OpenShift/Kubernetes data directly from Etcd.

## How to build

    $ go build .

## Basic Usage

This requires setting the following flags:

* `-key` - points to `master.etcd-client.key`
* `-cert` - points to `master.etcd-client.crt`
* `-cacert` - points to `ca.crt`

Once these are set properly, one can invoke the following actions:

* `ls` - list all keys starting with prefix
* `get` - get the specific value of a key
* `dump` - dump the entire contents of the etcd

## Sample Basic Usage

List all keys starting with `/openshift.io`:

```
etcdhelper -key master.etcd-client.key -cert master.etcd-client.crt -cacert ca.crt ls /openshift.io
```

Get JSON-representation of `imagestream/python` from `openshift` namespace:

```
etcdhelper -key master.etcd-client.key -cert master.etcd-client.crt -cacert ca.crt get /openshift.io/imagestreams/openshift/python
```

Dump the contents of etcd to stdout:

```
etcdhelper -key master.etcd-client.key -cert master.etcd-client.crt -cacert ca.crt dump
```

## Advanced Usage

If your OpenShift cluster has `etcd` [encryption enabled](https://docs.openshift.com/container-platform/4.9/security/encrypting-etcd.html), you will not be able to retrieve the values for these objcets in ETCD:

* `ConfigMaps`
* `Secrets`
* `Routes`
* `OAuth tokens*`

**NOTE** Currently this only supports encryption of the `aescbc` type. This is only noted in case future releases add in various encryption types.

Once these are set properly, one can invoke the following actions:

* `secrets` - get the decrypted value of an ecnrypted key

This requires setting the following flags:

* `-encryption-key`
* `-encryption-secret`

### Getting the Encryption Keys

To get these values you have a few options, but it depends on how you are using this (from an `etcd` backup or a live `etcd` cluster).

When `etcd` encryption is enabled you will not be able to retrieve values for the key types listed above. However, you can obtain the Encryption Key and Secret used if you have access to the `openshift-config-managed` namespace. These `Secrets` are rotated, so you will need to look for the latest (or highest numbered) `Secret`. Example using `jq` to trim down the `Secret`:

```sh
$ oc get secret -n openshift-config-managed encryption-config-openshift-kube-apiserver -o json | jq -r '.data."encryption-config"' | base64 -d | jq -r '.resources[0].providers[0].aescbc.keys'
[
  {
    "name": "1",
    "secret": "8kU3ejkS86Au/eLzQ4rBR//O1spU0Lbno1JzEBxI="
  }
]
```

If you are backing up `etcd` through the [supported method for OpenShift](https://docs.openshift.com/container-platform/4.9/backup_and_restore/control_plane_backup_and_restore/backing-up-etcd.html), the backup will contain an `etcd` snapshot and a `tarball` of static Kubernetes resources. The resources will contain the active encryption key/secret for the keys inside of the `etcd` snapshot. Once you extract the contents, you can run a simple `find` command to get what is needed:

```sh
$ find static-pod-resources -iname encryption-config* -type f -printf '%T+ %p\n' | sort | head -n 1 | awk '{print $2}'
static-pod-resources/kube-apiserver-pod-1/secrets/encryption-config/encryption-config
## Then run the above jq command on this file to get the encryption key/secret used for this backup
cat static-pod-resources/kube-apiserver-pod-1/secrets/encryption-config/encryption-config | jq -r '.data."encryption-config"' | base64 -d | jq -r '.resources[0].providers[0].aescbc.keys'
[
  {
    "name": "1",
    "secret": "8kU3ejkS86Au/eLzQ4rBR//O1spU0Lbno1JzEBxI="
  }
]
```

### Retrieve Encrypted Values

With this data in hand, you can now get the values and decrypt them so they can be read/used:

```sh
$ etcdhelper -encryption-key="1" -encryption-secret="8kU3ejkS86Au/eLzQ4rBR//O1spU0Lbno1JzEBxI=" -key master.etcd-client.key -cert master.etcd-client.crt -cacert ca.crt secrets /kubernetes.io/secrets/openshift-network-operator/default-dockercfg-1h8g7
apiVersion: v1
data:
  .dockercfg: eyIxMC4~~~rest of the data~~~ifx013= 
kind: Secret
metadata:
  ~~~~~
```

As opposed to what happens if you do not decrypt:

```sh
$ etcdhelper -encryption-key="1" -encryption-secret="8kU3ejkS86Au/eLzQ4rBR//O1spU0Lbno1JzEBxI=" -key master.etcd-client.key -cert master.etcd-client.crt -cacert ca.crt get /kubernetes.io/secrets/openshift-network-operator/default-dockercfg-1h8g7
WARN: unable to decode /kubernetes.io/secrets/openshift-network-operator/default-dockercfg-1h8g7: yaml: invalid leading UTF-8 octet
```
