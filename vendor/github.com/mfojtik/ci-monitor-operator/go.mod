module github.com/mfojtik/ci-monitor-operator

go 1.14

require (
	github.com/jteeuwen/go-bindata v3.0.8-0.20151023091102-a0ff2567cfb7+incompatible
	github.com/openshift/build-machinery-go v0.0.0-20200211121458-5e3d6e570160
	github.com/openshift/client-go v0.0.0-20200326155132-2a6cd50aedd0
	github.com/openshift/library-go v0.0.0-20200402123743-4015ba624cae
	github.com/prometheus/client_golang v1.5.1 // indirect
	github.com/spf13/cobra v0.0.7
	github.com/spf13/pflag v1.0.5
	gopkg.in/src-d/go-git.v4 v4.13.1
	k8s.io/apiextensions-apiserver v0.18.0
	k8s.io/apimachinery v0.18.0
	k8s.io/client-go v0.18.0
	k8s.io/component-base v0.18.0
	k8s.io/klog v1.0.0
	sigs.k8s.io/yaml v1.2.0
)
