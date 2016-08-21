#
# This is the official OpenShift Origin pod infrastructure image. It will stay running
# until terminated by a signal and is the heart of each running pod. It holds on to
# the network and IPC namespaces as containers come and go during the lifetime of the
# pod.
#
# The standard name for this image is openshift/origin-pod
#
FROM centos:centos7

## Install origin repo
RUN INSTALL_PKGS="centos-release-openshift-origin" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all

## Install packages
RUN INSTALL_PKGS="origin-pod" && \
    yum install -y $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all && \
    ln -s /usr/bin/pod /pod

USER 1001
LABEL io.k8s.display-name="OpenShift Origin Pod Infrastructure" \
      io.k8s.description="This is a component of OpenShift Origin and holds on to the shared Linux namespaces within a Pod."
ENTRYPOINT ["/pod"]
