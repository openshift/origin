#
# This is the base image from which all OpenShift Origin images inherit. Only packages
# common to all downstream images should be here.
#
# The standard name for this image is openshift/origin-base
#
FROM openshift/origin-source

RUN INSTALL_PKGS="bsdtar ceph-common device-mapper device-mapper-persistent-data e2fsprogs ethtool findutils git hostname iptables lsof nmap-ncat socat sysvinit-tools tar tree util-linux wget which xfsprogs" && \
    yum install -y ${INSTALL_PKGS} && \
    rpm -V ${INSTALL_PKGS} && \
    yum clean all && \
    mkdir -p /var/lib/origin

LABEL io.k8s.display-name="OpenShift Origin CentOS 7 Base" \
      io.k8s.description="This is the base image from which all OpenShift Origin images inherit." \
      io.openshift.tags="openshift,base"
