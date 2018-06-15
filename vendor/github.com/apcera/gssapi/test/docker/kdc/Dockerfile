FROM ubuntu:14.04
ADD krb5.conf /etc/krb5.conf

RUN apt-get -y update
RUN apt-get -y install heimdal-kdc

ADD entrypoint.sh /etc/docker-kdc/entrypoint.sh
EXPOSE	88
ENTRYPOINT /etc/docker-kdc/entrypoint.sh

