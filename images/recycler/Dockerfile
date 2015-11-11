#
# This is the default deployment strategy image for OpenShift Origin. It expects a set of
# environment variables to parameterize the deploy:
#
#   KUBERNETES_MASTER - the address of the OpenShift master
#   KUBERNETES_DEPLOYMENT_ID - the deployment identifier that is running this build
#
# The standard name for this image is openshift/origin-recycler
#
FROM openshift/origin

RUN yum install -y sudo && \
    sed -i -e "s/Defaults    requiretty.*/ #Defaults    requiretty/g" /etc/sudoers

ADD scripts/recycler.sh /usr/share/openshift/scripts/volumes/recycler.sh

ENTRYPOINT ["/usr/share/openshift/scripts/volumes/recycler.sh"]
