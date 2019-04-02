# Dockerfile to bootstrap build and test in openshift-ci
#

FROM openshift/origin-release:golang-1.10

# install tox using pip rather than rpm because the version
# in centos 7 (which our base image is based on) is
# too old and lacks features we use
RUN yum install -y glide python-pip python-virtualenv python36 git \
    && pip install tox

