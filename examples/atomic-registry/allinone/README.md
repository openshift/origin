# DEPRECATED

This is for reference only.

* Dedicated host registry deployments should reference the [systemd](../systemd/README.md) installation.
* For more flexibility in running clustered workloads, install OpenShift (single or multi-host) with integrated registry and then the Cockpit UI via template.

## Contributing to this Dockerfile source repo

Run build, install and test: `sudo make all-allinone`

## Requirements

- single host (laptop, vm, vagrant, etc.) with Docker
- Open TCP ports 8443, 443, 5000
- The hostname used during install will be the output of the `hostname` command. If that hostname does not resolve with DNS then pass the IP address to the install procedure.
- (optional) atomic cli, available on Red Hat-based systems Fedora, Centos, Red Hat Enterprise Linux, including Atomic host

## Install and Run

The install procedure should be run locally.

### With atomic CLI

1. Install the system service files and pull images.

        sudo atomic install projectatomic/atomic-registry-quickstart [hostname]
1. Optional: edit configuration file `/etc/origin/master/master-config.yaml`.
1. Run the application. This will enable and start the docker containers as system services.

        sudo atomic run projectatomic/atomic-registry-quickstart [hostname]

### With straight Docker

Replace steps 1 and 3 above with the output of the inspect command.

    sudo docker inspect -f '{{ .Config.Labels.INSTALL }}' projectatomic/atomic-registry-quickstart
    sudo docker inspect -f '{{ .Config.Labels.RUN }}' projectatomic/atomic-registry-quickstart

This will provide the docker run commands to install and run the registry installation.

If you make changes to the API  configuration file `/etc/origin/master/master-config.yaml` restart the API service.

    sudo docker restart origin

## Try it out

1. Explore the web UI on https://<hostname>
1. Login with docker using the reference commands, build and push an image.

## Uninstall

    sudo atomic uninstall --force projectatomic/atomic-registry-quickstart

# Optional Setup steps

1. [Configure authentication](https://docs.openshift.org/latest/install_config/configuring_authentication.html). Restart the origin API server after making changes to the config file: `sudo docker restart origin`
1. [Configure persistent registry storage](https://docs.openshift.org/latest/install_config/install/docker_registry.html#advanced-overriding-the-registry-configuration)
1. [Assign a user cluster-admin privilege](https://docs.openshift.org/latest/admin_guide/manage_authorization_policy.html#managing-role-bindings)
1. Explore the web UI

## Reference Documentation

https://docs.openshift.org/latest/welcome/index.html
