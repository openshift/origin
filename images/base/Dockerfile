#
# This is the base image from which all OpenShift Origin images inherit. Only packages
# common to all downstream images should be here.
#
# The standard name for this image is openshift/origin-base
#
FROM openshift/origin-source

COPY *.repo /etc/yum.repos.d/
RUN INSTALL_PKGS=" \
      which tar wget hostname sysvinit-tools util-linux \
      socat tree findutils lsof bind-utils \
      " && \
    yum install -y ${INSTALL_PKGS} && \
    rpm -V ${INSTALL_PKGS} && \
    yum clean all && \
    mkdir -p /var/lib/origin

LABEL io.k8s.display-name="OpenShift Origin CentOS 7 Base" \
      io.k8s.description="This is the base image from which all OpenShift Origin images inherit." \
      io.openshift.tags="openshift,base"
