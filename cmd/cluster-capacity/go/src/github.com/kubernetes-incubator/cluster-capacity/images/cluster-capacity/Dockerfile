#
# This is the cluster capacity tool.
#
# The standard name for this image is openshift/origin-cluster-capacity
#
FROM centos:centos7

COPY bin/hypercc /bin/hypercc
RUN ln -sf /bin/hypercc /bin/cluster-capacity
RUN ln -sf /bin/hypercc /bin/genpod
CMD ["/bin/cluster-capacity --help"]
