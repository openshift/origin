#
# This is the default deployment strategy image for OpenShift Origin. It expects a set of
# environment variables to parameterize the deploy:
#
#   "OPENSHIFT_DEPLOYMENT_NAME" - the name of a replication controller that is being deployed
#   "OPENSHIFT_DEPLOYMENT_NAMESPACE" - the namespace of the replication controller that is being deployed
#
# It also expects to receive the standard Kubernetes service account secret to connect back to
# the OpenShift API to drive the deployment.
#
# The standard name for this image is openshift/origin-deployer
#
FROM openshift/origin

LABEL io.k8s.display-name="OpenShift Origin Deployer" \
      io.k8s.description="This is a component of OpenShift Origin and executes the user deployment process to roll out new containers. It may be used as a base image for building your own custom deployer image." \
      io.openshift.tags="openshift,deployer"
# The deployer doesn't require a root user.
USER 1001
ENTRYPOINT ["/usr/bin/openshift-deploy"]
