# Atomic Registry

## Deployments

### Systemd

Systemd-managed containers is the simplest deployment method of Atomic Registry.

* Installer image: https://hub.docker.com/r/projectatomic/atomic-registry-install/
* Installation Dockerfile source: [systemd](systemd/)
* Documentation: http://docs.projectatomic.io/registry/

### All-in-One: DEPRECATED

The OpenShift all-in-one deployment method (formerly "Quickstart") has been deprecated. Current recommendation is to install OpenShift and integrated registry and then the Cockpit UI via template. Deploying an OpenShift environment provides the most flexibility for running clustered workloads.

* Installer image: https://hub.docker.com/r/projectatomic/atomic-registry-quickstart/
* Installation Dockerfile source: [allinone](allinone/)
