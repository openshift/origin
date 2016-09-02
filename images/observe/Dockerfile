#
# This is the observer image for OpenShift Origin that makes it easy to script a reaction
# to changes on the cluster. It uses the `oc observe` command and expects to be run inside
# of a Kubernetes pod or have security information set via KUBECONFIG and a bind mounted
# kubeconfig file.
#
# The standard name for this image is openshift/observe
#
FROM openshift/origin

LABEL io.k8s.display-name="OpenShift Observer" \
      io.k8s.description="This image runs the oc observe command to watch and react to changes on your cluster."
# The observer doesn't require a root user.
USER 1001
ENTRYPOINT ["/usr/bin/oc", "observe"]
