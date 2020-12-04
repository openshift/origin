FROM image-registry.openshift-image-registry.svc:5000/openshift/tools:latest
USER root

ADD .s2i/bin/assemble .
RUN ./assemble

# exit 1 causes the docker build to fail which causes docker to show the output # of all commands like 'assemble' above.
RUN exit 1

