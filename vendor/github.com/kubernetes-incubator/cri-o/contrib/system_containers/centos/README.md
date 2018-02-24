# cri-o

This is the cri-o daemon as a system container.

## Building the image from source:

```
# git clone https://github.com/projectatomic/atomic-system-containers
# cd atomic-system-containers/cri-o
# docker build -t crio .
```

## Running the system container, with the atomic CLI:

Pull from registry into ostree:

```
# atomic pull --storage ostree $REGISTRY/crio
```

Or alternatively, pull from local docker:

```
# atomic pull --storage ostree docker:crio:latest
```

Install the container:

Currently we recommend using --system-package=no to avoid having rpmbuild create an rpm file
during installation. This flag will tell the atomic CLI to fall back to copying files to the
host instead.

```
# atomic install --system --system-package=no --name=crio ($REGISTRY)/crio
```

Start as a systemd service:

```
# systemctl start crio
```

Stopping the service

```
# systemctl stop crio
```

Removing the container

```
# atomic uninstall crio
```

## Binary version

You can find the image automatically built as: registry.centos.org/projectatomic/cri-o:latest
