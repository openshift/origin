FROM registry.svc.ci.openshift.org/ocp/4.2:cli

LABEL io.k8s.display-name="OpenShift Deployer" \
      io.k8s.description="This is a component of OpenShift and executes the user deployment process to roll out new containers. It may be used as a base image for building your own custom deployer image." \
      io.openshift.tags="openshift,deployer"
# The deployer doesn't require a root user.
USER 1001
ENTRYPOINT ["/usr/bin/openshift-deploy"]
