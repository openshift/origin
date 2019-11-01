# Image to launch GCE from OpenShift for CI

This image enables launching a GCE instance from OpenShift, for CI testing.

This image contains [`nss_wrapper`](https://cwrap.org/nss_wrapper.html) to execute `ssh` commands as
a mock user to interact with a GCE instance from an OpenShift container.

OpenShift containers run with an arbitrary uid, but SSH requires a valid user.  `nss_wrapper`
allows for the container's user ID to be mapped to a username inside of a container.

### Example Usage

You can override the container's current user ID and group ID by providing `NSS_WRAPPER_GROUP`
and `NSS_WRAPPER_PASSWD` for the mock files, as well as `NSS_USERNAME`, `NSS_UID`, `NSS_GROUPNAME`,
and/or `NSS_GID`. In OpenShift CI, `NSS_USERNAME` and `NSS_GROUPNAME` are set.
The random UID assigned to the container is the UID that the mock username is mapped to.

```console
$ podman run --rm \
>   -e NSS_WRAPPER_GROUP=/tmp/group \
>   -e NSS_WRAPPER_PASSWD=/tmp/passwd \
>   -e NSS_UID=1000 \
>   -e NSS_GID=1000 \
>   -e NSS_USERNAME=testuser \
>   -e NSS_GROUPNAME=testuser \
>   nss_wrapper_img mock-nss.sh id testuser
uid=1000(testuser) gid=1000(testuser) groups=1000(testuser)
```

Or, in an OpenShift container:

```yaml
containers:
- name: setup
  image: nss-wrapper-image
  env:
  - name: NSS_WRAPPER_PASSWD
    value: /tmp/passwd
  - name: NSS_WRAPPER_GROUP
    value: /tmp/group
  - name: NSS_USERNAME
    value: mockuser
  - name: NSS_GROUPNAME
    value: mockuser
  command:
  - /bin/sh
  - -c
  - |
    #!/bin/sh
    mock-nss.sh
    LD_PRELOAD=/usr/lib64/libnss_wrapper.so gcloud compute scp [gcloud scp args]
```
