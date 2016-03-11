FROM centos/ruby-22-centos7:latest
USER root
RUN yum -y install expect
RUN echo "root:redhat" | chpasswd
USER 1001
COPY ./adduser /usr/libexec/s2i/
COPY ./assemble /usr/libexec/s2i/
