FROM lyft/envoy-alpine-debug:latest
ADD bootstrap.yaml .
CMD ["/usr/local/bin/envoy", "-c", "bootstrap.yaml"]