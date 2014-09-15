FROM openshift/kubernetes-fedora-dind
RUN yum -y install git
ADD ./build.sh /tmp/build.sh
CMD ["/tmp/build.sh"]
