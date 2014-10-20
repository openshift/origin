FROM centos:centos7
RUN yum -y install git http://copr-be.cloud.fedoraproject.org/results/goldmann/docker-io/epel-7-x86_64/docker-io-1.2.0-2.1.fc20/docker-io-1.2.0-2.1.el7.centos.x86_64.rpm
ENV HOME /root
ADD ./build.sh /tmp/build.sh
CMD ["/tmp/build.sh"]
