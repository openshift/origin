# Instructions to install API using the installer
# Build and test the service-idler
FROM golang:1.9.3 as builder

ENV TEST_ASSET_DIR /usr/local/bin
ENV TEST_ASSET_KUBECTL $TEST_ASSET_DIR/kubectl
ENV TEST_ASSET_KUBE_APISERVER $TEST_ASSET_DIR/kube-apiserver
ENV TEST_ASSET_ETCD $TEST_ASSET_DIR/etcd

# Download test framework binaries
ENV TEST_ASSET_URL https://storage.googleapis.com/k8s-c10s-test-binaries
RUN curl ${TEST_ASSET_URL}/etcd-Linux-x86_64 --output $TEST_ASSET_ETCD
RUN curl ${TEST_ASSET_URL}/kube-apiserver-Linux-x86_64 --output $TEST_ASSET_KUBE_APISERVER
RUN curl https://storage.googleapis.com/kubernetes-release/release/v1.9.2/bin/linux/amd64/kubectl --output $TEST_ASSET_KUBECTL
RUN chmod +x $TEST_ASSET_ETCD
RUN chmod +x $TEST_ASSET_KUBE_APISERVER
RUN chmod +x $TEST_ASSET_KUBECTL

# Copy in the go src
WORKDIR /go/src/github.com/openshift/service-idler
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# Build and test the API code
RUN CGO_ENABLED=0 go build -a -o service-idler ./cmd/service-idler/main.go
RUN go test ./pkg/... ./cmd/...

# Copy the service-idler into a thin image
FROM scratch
WORKDIR /root/
COPY --from=builder /go/src/github.com/openshift/service-idler/service-idler .
ENTRYPOINT ["./service-idler", "--logtostderr"]
CMD ["--install-crds=false"]  
