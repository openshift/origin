#
# This is the official kube-proxy image.
#
# The standard name for this image is openshift/origin-proxy
#
FROM openshift/origin-hyperkube

RUN INSTALL_PKGS="iptables conntrack-tools" && \
    yum --enablerepo=origin-local-release install -y ${INSTALL_PKGS} && \
    rpm -V ${INSTALL_PKGS} && \
    yum clean all && rm -rf /var/cache/*

LABEL io.k8s.display-name="Kubernetes kube-proxy" \
      io.k8s.description="Provides kube-proxy for external CNI plugins" \
      io.openshift.tags="openshift,kube-proxy" \
      io.openshift.build.versions="kubernetes=1.12.4"
