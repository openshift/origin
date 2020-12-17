module github.com/openshift/origin

go 1.15

require (
	github.com/MakeNowJust/heredoc v0.0.0-20170808103936-bb23615498cd
	github.com/RangelReale/osincli v0.0.0-20160924135400-fababb0555f2
	github.com/apparentlymart/go-cidr v1.1.0
	github.com/auth0/go-jwt-middleware v0.0.0-20201030150249-d783b5c46b39 // indirect
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/distribution v2.7.1+incompatible
	github.com/fsouza/go-dockerclient v0.0.0-20171004212419-da3951ba2e9e
	github.com/ghodss/yaml v1.0.0
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/go-ozzo/ozzo-validation v3.5.0+incompatible // indirect
	github.com/golang/protobuf v1.4.3
	github.com/google/go-cmp v0.5.2
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/google/uuid v1.1.2
	github.com/gorilla/context v1.1.1 // indirect
	github.com/heketi/tests v0.0.0-20151005000721-f3775cbcefd6 // indirect
	github.com/lestrrat-go/jspointer v0.0.0-20181205001929-82fadba7561c // indirect
	github.com/lestrrat-go/jsref v0.0.0-20181205001954-1b590508f37d // indirect
	github.com/lestrrat-go/jsschema v0.0.0-20181205002244-5c81c58ffcc3 // indirect
	github.com/lestrrat-go/jsval v0.0.0-20181205002323-20277e9befc0 // indirect
	github.com/lestrrat-go/pdebug v0.0.0-20200204225717-4d6bd78da58d // indirect
	github.com/lestrrat-go/structinfo v0.0.0-20190212233437-acd51874663b // indirect
	github.com/lestrrat/go-jsschema v0.0.0-20181205002244-5c81c58ffcc3
	github.com/lpabon/godbc v0.1.1 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/onsi/ginkgo v4.5.0-origin.1+incompatible
	github.com/onsi/gomega v1.7.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/openshift/api v0.0.0-20201214114959-164a2fb63b5f
	github.com/openshift/apiserver-library-go v0.0.0-20201214145556-6f1013f42f98
	github.com/openshift/build-machinery-go v0.0.0-20200917070002-f171684f77ab
	github.com/openshift/client-go v0.0.0-20201214125552-e615e336eb49
	github.com/openshift/library-go v0.0.0-20201214135256-d265f469e75b
	github.com/pborman/uuid v1.2.0
	github.com/pquerna/cachecontrol v0.0.0-20201205024021-ac21108117ac // indirect
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.10.0
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.0
	github.com/stretchr/objx v0.2.0
	github.com/stretchr/testify v1.6.1
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	go.etcd.io/etcd v0.5.0-alpha.5.0.20200910180754-dd1b699fc489
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	google.golang.org/grpc v1.27.1
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.20.0
	k8s.io/apiextensions-apiserver v0.20.0
	k8s.io/apimachinery v0.20.0
	k8s.io/apiserver v0.20.0
	k8s.io/cli-runtime v0.20.0
	k8s.io/client-go v0.20.0
	k8s.io/component-base v0.20.0
	k8s.io/component-helpers v0.0.0
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20201113171705-d219536bb9fd
	k8s.io/kubectl v0.20.0
	k8s.io/kubelet v0.20.0
	k8s.io/kubernetes v1.20.0
	k8s.io/legacy-cloud-providers v0.20.0
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/onsi/ginkgo => github.com/openshift/onsi-ginkgo v4.5.0-origin.1+incompatible
	github.com/openshift/apiserver-library-go => github.com/openshift/apiserver-library-go v0.0.0-20201214145556-6f1013f42f98
	github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20201214125552-e615e336eb49
	github.com/openshift/library-go => github.com/openshift/library-go v0.0.0-20201214135256-d265f469e75b
	k8s.io/api => github.com/openshift/kubernetes/staging/src/k8s.io/api v0.0.0-20201215095843-87544c5b79d2
	k8s.io/apiextensions-apiserver => github.com/openshift/kubernetes/staging/src/k8s.io/apiextensions-apiserver v0.0.0-20201215095843-87544c5b79d2
	k8s.io/apimachinery => github.com/openshift/kubernetes/staging/src/k8s.io/apimachinery v0.0.0-20201215095843-87544c5b79d2
	k8s.io/apiserver => github.com/openshift/kubernetes/staging/src/k8s.io/apiserver v0.0.0-20201215095843-87544c5b79d2
	k8s.io/cli-runtime => github.com/openshift/kubernetes/staging/src/k8s.io/cli-runtime v0.0.0-20201215095843-87544c5b79d2
	k8s.io/client-go => github.com/openshift/kubernetes/staging/src/k8s.io/client-go v0.0.0-20201215095843-87544c5b79d2
	k8s.io/cloud-provider => github.com/openshift/kubernetes/staging/src/k8s.io/cloud-provider v0.0.0-20201215095843-87544c5b79d2
	k8s.io/cluster-bootstrap => github.com/openshift/kubernetes/staging/src/k8s.io/cluster-bootstrap v0.0.0-20201215095843-87544c5b79d2
	k8s.io/code-generator => github.com/openshift/kubernetes/staging/src/k8s.io/code-generator v0.0.0-20201215095843-87544c5b79d2
	k8s.io/component-base => github.com/openshift/kubernetes/staging/src/k8s.io/component-base v0.0.0-20201215095843-87544c5b79d2
	k8s.io/component-helpers => github.com/openshift/kubernetes/staging/src/k8s.io/component-helpers v0.0.0-20201215095843-87544c5b79d2
	k8s.io/controller-manager => github.com/openshift/kubernetes/staging/src/k8s.io/controller-manager v0.0.0-20201215095843-87544c5b79d2
	k8s.io/cri-api => github.com/openshift/kubernetes/staging/src/k8s.io/cri-api v0.0.0-20201215095843-87544c5b79d2
	k8s.io/csi-translation-lib => github.com/openshift/kubernetes/staging/src/k8s.io/csi-translation-lib v0.0.0-20201215095843-87544c5b79d2
	k8s.io/gengo => k8s.io/gengo v0.0.0-20200114144118-36b2048a9120
	k8s.io/heapster => k8s.io/heapster v1.2.0-beta.1
	k8s.io/klog => k8s.io/klog v1.0.0
	k8s.io/kube-aggregator => github.com/openshift/kubernetes/staging/src/k8s.io/kube-aggregator v0.0.0-20201215095843-87544c5b79d2
	k8s.io/kube-controller-manager => github.com/openshift/kubernetes/staging/src/k8s.io/kube-controller-manager v0.0.0-20201215095843-87544c5b79d2
	k8s.io/kube-proxy => github.com/openshift/kubernetes/staging/src/k8s.io/kube-proxy v0.0.0-20201215095843-87544c5b79d2
	k8s.io/kube-scheduler => github.com/openshift/kubernetes/staging/src/k8s.io/kube-scheduler v0.0.0-20201215095843-87544c5b79d2
	k8s.io/kubectl => github.com/openshift/kubernetes/staging/src/k8s.io/kubectl v0.0.0-20201215095843-87544c5b79d2
	k8s.io/kubelet => github.com/openshift/kubernetes/staging/src/k8s.io/kubelet v0.0.0-20201215095843-87544c5b79d2
	k8s.io/kubernetes => github.com/openshift/kubernetes v1.20.1-0.20201215095843-87544c5b79d2
	k8s.io/legacy-cloud-providers => github.com/openshift/kubernetes/staging/src/k8s.io/legacy-cloud-providers v0.0.0-20201215095843-87544c5b79d2
	k8s.io/metrics => github.com/openshift/kubernetes/staging/src/k8s.io/metrics v0.0.0-20201215095843-87544c5b79d2
	k8s.io/mount-utils => github.com/openshift/kubernetes/staging/src/k8s.io/mount-utils v0.0.0-20201215095843-87544c5b79d2
	k8s.io/sample-apiserver => github.com/openshift/kubernetes/staging/src/k8s.io/sample-apiserver v0.0.0-20201215095843-87544c5b79d2
	k8s.io/sample-cli-plugin => github.com/openshift/kubernetes/staging/src/k8s.io/sample-cli-plugin v0.0.0-20201215095843-87544c5b79d2
	k8s.io/sample-controller => github.com/openshift/kubernetes/staging/src/k8s.io/sample-controller v0.0.0-20201215095843-87544c5b79d2
	k8s.io/system-validators => k8s.io/system-validators v1.0.4
)
