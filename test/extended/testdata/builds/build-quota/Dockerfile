FROM registry.redhat.io/ubi8/ubi
USER root

ADD .s2i/bin/assemble .
# test fixtures are always recreated with mode 0o640, so we have to chmod +x here
RUN chmod +x ./assemble && ./assemble

# exit 1 causes the docker build to fail which causes docker to show the output # of all commands like 'assemble' above.
RUN exit 1
