FROM registry.svc.ci.openshift.org/ocp/4.2:cli

LABEL io.k8s.display-name="OpenShift Volume Recycler" \
      io.k8s.description="This is a component of OpenShift and is used to prepare persistent volumes for reuse after they are deleted." \
      io.openshift.tags="openshift,recycler"
ENTRYPOINT ["/usr/bin/openshift-recycle"]
