% CRI-O (1) Container Image Pages
% Jhon Honce
% September 7, 2017

# NAME
cri-o - OCI-based implementation of Kubernetes Container Runtime Interface

# DESCRIPTION
CRI-O is an implementation of the Kubernetes CRI. It is a lightweight, OCI-compliant runtime that is native to kubernetes. CRI-O supports OCI container images and can pull from any container registry.

You can find more information on the CRI-O project at <https://github.com/kubernetes-incubator/cri-o/>

# USAGE
Pull from local docker and install system container:

```
# atomic pull --storage ostree docker:openshift3/cri-o:latest
# atomic install --system --system-package=no --name cri-o openshift3/cri-o
```

Start and enable as a systemd service:
```
# systemctl enable --now cri-o
```

Stopping the service
```
# systemctl stop cri-o
```

Removing the container
```
# atomic uninstall cri-o
```

# SEE ALSO
man systemd(1)
