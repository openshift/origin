FROM ubuntu:14.04
RUN apt-get -y update
RUN apt-get -y install \
        gcc \
        libgssapi-krb5-2 \
        libkrb5-dev \
        libsasl2-modules-gssapi-mit \
        wget

RUN (cd /tmp && wget https://storage.googleapis.com/golang/go1.5.linux-amd64.tar.gz && tar xvf go1.5.linux-amd64.tar.gz && mv go/ /opt)
ENV GOROOT="/opt/go"
ADD krb5.keytab /opt/go-gssapi-test-service/krb5.keytab
ADD krb5.conf.template /tmp/krb5.conf.template
ADD entrypoint.sh /opt/go-gssapi-test-service/entrypoint.sh

EXPOSE 80
ENTRYPOINT /opt/go-gssapi-test-service/entrypoint.sh
