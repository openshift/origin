module github.com/openshift/client-go

go 1.13

require (
	github.com/openshift/api v0.0.0-20191213091414-3fbf6bcf78e8
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v0.17.0
	k8s.io/code-generator v0.17.0
)

replace (
	github.com/openshift/api => github.com/openshift/api v0.0.0-20191213091414-3fbf6bcf78e8
	k8s.io/code-generator => github.com/openshift/kubernetes-code-generator v0.0.0-20191216140939-db549faca3fe
)
