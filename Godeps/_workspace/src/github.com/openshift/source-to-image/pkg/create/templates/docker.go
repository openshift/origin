package templates

const Dockerfile = `
# {{.ImageName}}
FROM openshift/base-centos7

ENV STI_NODEJS_VERSION 0.10

# TODO: Install required packages here:
# RUN yum install -y ... ; yum clean all -y

USER default

# TODO (optional): Copy the builder files into /opt/openshift
# COPY ./<builder_folder>/ /opt/openshift/

# TODO: Set the default port for applications built using this image
# EXPOSE 3000
`
