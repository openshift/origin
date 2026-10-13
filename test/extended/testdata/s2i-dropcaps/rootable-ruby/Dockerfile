FROM registry.access.redhat.com/ubi8/ruby-27:latest
USER root
RUN rm -f /usr/bin/ls
RUN echo "root:redhat" | chpasswd
USER 1001
COPY ./adduser /usr/libexec/s2i/
COPY ./assemble /usr/libexec/s2i/
