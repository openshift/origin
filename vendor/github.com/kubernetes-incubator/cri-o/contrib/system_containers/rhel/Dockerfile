#oit## This file is managed by the OpenShift Image Tool
#oit## by the OpenShift Continuous Delivery team.
#oit##
#oit## Any yum repos listed in this file will effectively be ignored during CD builds.
#oit## Yum repos must be enabled in the oit configuration files.
#oit## Some aspects of this file may be managed programmatically. For example, the image name, labels (version,
#oit## release, and other), and the base FROM. Changes made directly in distgit may be lost during the next
#oit## reconciliation.
#oit##
FROM rhel7:7-released

RUN \
    yum install --setopt=tsflags=nodocs -y socat iptables cri-o iproute runc skopeo-containers container-selinux && \
    rpm -V socat iptables cri-o iproute runc skopeo-containers container-selinux && \
    yum clean all && \
    mkdir -p /exports/hostfs/etc/crio /exports/hostfs/opt/cni/bin/ /exports/hostfs/var/lib/containers/storage/ && \
    cp /etc/crio/* /exports/hostfs/etc/crio && \
    if test -e /usr/libexec/cni; then cp -Lr /usr/libexec/cni/* /exports/hostfs/opt/cni/bin/; fi

COPY manifest.json tmpfiles.template config.json.template service.template /exports/

COPY set_mounts.sh /
COPY run.sh /usr/bin/

CMD ["/usr/bin/run.sh"]

LABEL \
        com.redhat.component="cri-o-docker" \
        io.k8s.description="CRI-O is an implementation of the Kubernetes CRI. It is a lightweight, OCI-compliant runtime that is native to kubernetes. CRI-O supports OCI container images and can pull from any container registry." \
        maintainer="Jhon Honce <jhonce@redhat.com>" \
        name="openshift3/cri-o" \
        License="GPLv2+" \
        io.k8s.display-name="CRI-O" \
        summary="OCI-based implementation of Kubernetes Container Runtime Interface" \
        release="0.13.0.0" \
        version="v3.8.0" \
        architecture="x86_64" \
        usage="atomic install --system --system-package=no crio && systemctl start crio" \
        vendor="Red Hat" \
        io.openshift.tags="cri-o system rhel7" \
        atomic.type="system"
