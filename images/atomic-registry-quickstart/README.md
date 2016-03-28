# Getting Started With Atomic Registry

http://projectatomic.io/registry

**Requirements**

- Red Hat-based system (RHEL, Centos, Fedora, including Atomic)
- Docker
- atomic cli

## Install and Run

1. Install the system service files and pull images.

        sudo atomic install atomic-registry-quickstart
1. Optional: edit configuration file `/etc/origin/master/master-config.yaml`.
1. Run the application. This will enable and start the docker containers as system services.

        sudo atomic run atomic-registry-quickstart

## Stopping the application

`sudo atomic stop atomic-registry-quickstart`

## Uninstall

`sudo atomic uninstall atomic-registry-quickstart`

# Additional Setup steps

1. [Configure authentication](https://docs.openshift.org/latest/install_config/configuring_authentication.html)
1. [Configure persistent registry storage](https://docs.openshift.org/latest/install_config/install/docker_registry.html#advanced-overriding-the-registry-configuration)
1. [Assign a user cluster-admin privilege](https://docs.openshift.org/latest/admin_guide/manage_authorization_policy.html#managing-role-bindings)
1. Explore the web UI
1. Create a project and [push an image](https://docs.openshift.org/latest/install_config/install/docker_registry.html#access-logging-in-to-the-registry)

## Reference Documentation

https://docs.openshift.org/latest/welcome/index.html


