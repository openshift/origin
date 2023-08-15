FROM docker.io/fedora:38
COPY ./openshift-tests /usr/bin/openshift-tests

LABEL io.k8s.display-name="FAKE OpenShift End-to-End Tests" \
      io.k8s.description="FAKE OpenShift is a platform for developing, building, and deploying containerized applications." \
      io.openshift.tags="FAKE openshift,tests,e2e"
