module github.com/openshift/client-go

go 1.13

require (
	github.com/openshift/api v0.0.0-20200116145750-0e2ff1e215dd
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.17.1
	k8s.io/apimachinery v0.17.1
	k8s.io/client-go v0.17.1
	k8s.io/code-generator v0.17.1
)

// needed for pluralization patches open upstream.  Remove in v0.18.0
replace k8s.io/code-generator => github.com/openshift/kubernetes-code-generator v0.0.0-20191216140939-db549faca3fe
