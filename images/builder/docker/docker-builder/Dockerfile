FROM fedora:20
RUN yum -y install docker-io git
ENV HOME /root
ADD ./build.sh /tmp/build.sh
CMD ["/tmp/build.sh"]
