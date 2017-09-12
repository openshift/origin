#!/bin/bash

set -e

glide cache-clear
glide update


#  all the ugorji's need to agree or we end up failing.  Yours, mine, and ours fail!
rm -rf ./vendor/k8s.io/api/vendor/github.com/ugorji
rm -rf ./vendor/k8s.io/client-go/vendor/github.com/ugorji
rm -rf ./vendor/k8s.io/apiserver/vendor/github.com/ugorji
rm -rf ./vendor/k8s.io/apimachinery/vendor/github.com/ugorji

#  kube-openapi and apiserver need to agree on github.com/emicklei/go-restful or we fail to build.  Yours, mine, and ours fail!
rm -rf ./vendor/k8s.io/apiserver/vendor/github.com/emicklei

#  our package and apiserver have to agree on github.com/spf13/pflag or we fail to build.  Yours, mine, and ours fail!
rm -rf ./vendor/k8s.io/apiserver/vendor/github.com/spf13/pflag

