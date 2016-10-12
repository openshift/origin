#
# Image configured for installation of origin via openshift-ansible.
#
# The standard name for this image is openshift/dind-host
#
# Configure the docker host as follows:
#
#  - disable SELinux (not compatible with dind)
#
#  - disable iptables processing of packets traversing bridges
#
#      $ sysctl -w net.bridge.bridge-nf-call-iptables=0
#
#  - load the openvswitch module
#
#      $ modprobe openvswitch
#
#  - load the br_netfilter module
#
#      $ modprobe br_netfilter
#
#  - to use the overlay graphdriver, load the overlay module
#
#      $ modprobe overlay
#
# Run the container with --privileged
#
#      $ docker run -d --privileged openshift/dind-host
#

FROM openshift/dind:centos7

RUN yum -y update && yum -y install\
 NetworkManager\
 bash-completion\
 bind-utils\
 bridge-utils\
 ethtool\
 findutils\
 git\
 hostname\
 iproute\
 iptables-services\
 iputils\
 less\
 net-tools\
 procps-ng\
 python\
 tar\
 wget\
 which\
 && yum clean all

# Required by openshift-ansible
RUN systemctl enable NetworkManager

# Perform this post-build if the services network is not the cidr below
RUN echo "INSECURE_REGISTRY='--insecure-registry 172.30.0.0/16'" >\
 /etc/sysconfig/docker
