# How to regenerate vendor/ with independent grpc

* hack/godep-restore.sh
* cd $GOPATH/src/google.golang.org/grpc && git checkout 231b4cfea0e79843053a33f5fe90bd4d84b23cd3 && cd -
* cd vendor/k8s.io/kubernetes/pkg/kubelet/api
* rm -rf vendor/ Godeps/
* temporarily commit to get a clean repo
* GOPATH=$GOPATH:$GOPATH/src/github.com/openshift/origin/vendor/k8s.io/kubernetes/staging godep save ./...
* delete everything from vendor/ other than vendor/google.golang.org/grpc
* delete everything from Godeps.json other than grpc google.golang.org/grpc
