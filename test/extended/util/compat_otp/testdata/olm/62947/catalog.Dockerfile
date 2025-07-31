FROM registry.redhat.io/openshift4/ose-operator-registry:v4.12

ENTRYPOINT ["/bin/opm"]
CMD ["serve", "/configs"]

ADD catalog /configs

LABEL operators.operatorframework.io.index.configs.v1=/configs
