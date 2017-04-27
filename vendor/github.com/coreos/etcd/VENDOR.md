# How to regenerate vendor/ with independent grpc

* hack/godep-restore.sh
* cd $GOPATH/src/google.golang.org/grpc && git checkout 231b4cfea0e79843053a33f5fe90bd4d84b23cd3 && cd -
* cd vendor/github.com/coreos/etcd
* rm -rf vendor/ Godeps/
* temporarily commit to get a clean repo
* GOPATH=$GOPATH:$GOPATH/src/github.com/openshift/origin/vendor/k8s.io/kubernetes/staging godep save ./...
* delete everything from vendor/ other than google.golang.org/grpc and
  github.com/grpc-ecosystem/grpc-gateway
* delete everything from Godeps.json other than google.golang.org/grpc and
  github.com/grpc-ecosystem/grpc-gateway
