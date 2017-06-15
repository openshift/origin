# This is the cluster capacity tool.
#
# The standard name for this image is openshift/origin-cluster-capacity
#
FROM openshift/origin-source

RUN INSTALL_PKGS="origin-cluster-capacity" && \
    yum --enablerepo=origin-local-release install -y ${INSTALL_PKGS} && \
    rpm -V ${INSTALL_PKGS} && \
    yum clean all

LABEL io.k8s.display-name="OpenShift Origin Cluster Capacity" \
      io.k8s.description="This is a component of OpenShift Origin and runs cluster capacity analysis tool."

CMD ["/usr/bin/cluster-capacity --help"]
