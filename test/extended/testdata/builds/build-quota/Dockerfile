FROM centos:7
USER root

ADD .s2i/bin/assemble .
RUN ./assemble

# exit 1 causes the docker build to fail which causes docker to show the output # of all commands like 'assemble' above.
RUN exit 1

