module github.com/openshift/origin

go 1.16

require (
	github.com/MakeNowJust/heredoc v0.0.0-20170808103936-bb23615498cd
	github.com/RangelReale/osincli v0.0.0-20160924135400-fababb0555f2
	github.com/apparentlymart/go-cidr v1.1.0
	github.com/auth0/go-jwt-middleware v0.0.0-20201030150249-d783b5c46b39 // indirect
	github.com/blang/semver v3.5.1+incompatible
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/distribution v2.7.1+incompatible
	github.com/fsouza/go-dockerclient v1.7.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/go-ozzo/ozzo-validation v3.5.0+incompatible // indirect
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.5
	github.com/google/uuid v1.1.2
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
	github.com/onsi/ginkgo v4.7.0-origin.0+incompatible
	github.com/onsi/gomega v1.10.1
	github.com/opencontainers/go-digest v1.0.0
	github.com/openshift/api v0.0.0-20220110171111-997c316db5e1
	github.com/openshift/apiserver-library-go v0.0.0-20211209162547-8c11dbc46b6e
	github.com/openshift/build-machinery-go v0.0.0-20211213093930-7e33a7eb4ce3
	github.com/openshift/client-go v0.0.0-20211209144617-7385dd6338e3
	github.com/openshift/library-go v0.0.0-20220111125907-7f25b9c7ad22
	github.com/pborman/uuid v1.2.0
	github.com/pquerna/cachecontrol v0.0.0-20201205024021-ac21108117ac // indirect
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.28.0
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.1
	github.com/stretchr/objx v0.2.0
	github.com/stretchr/testify v1.7.0
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	go.etcd.io/etcd/api/v3 v3.5.0
	go.etcd.io/etcd/client/pkg/v3 v3.5.0
	go.etcd.io/etcd/client/v3 v3.5.0
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	golang.org/x/net v0.0.0-20211209124913-491a49abca63
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f
	google.golang.org/grpc v1.40.0
	gopkg.in/ldap.v2 v2.5.1
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.23.0
	k8s.io/apiextensions-apiserver v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/apiserver v0.23.0
	k8s.io/cli-runtime v0.23.0
	k8s.io/client-go v0.23.0
	k8s.io/component-base v0.23.0
	k8s.io/component-helpers v0.0.0
	k8s.io/klog/v2 v2.30.0
	k8s.io/kube-aggregator v0.23.0
	k8s.io/kube-openapi v0.0.0-20211115234752-e816edb12b65
	k8s.io/kubectl v0.23.0
	k8s.io/kubelet v0.23.0
	k8s.io/kubernetes v1.23.0
	k8s.io/legacy-cloud-providers v0.23.0
	k8s.io/utils v0.0.0-20211208161948-7d6a63dca704
	sigs.k8s.io/kustomize/kyaml v0.13.0
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/google/cadvisor => github.com/google/cadvisor v0.43.0
	github.com/onsi/ginkgo => github.com/openshift/onsi-ginkgo v4.7.0-origin.0+incompatible
	k8s.io/api => github.com/openshift/kubernetes/staging/src/k8s.io/api v0.0.0-20220130045626-42a86de5afdd
	k8s.io/apiextensions-apiserver => github.com/openshift/kubernetes/staging/src/k8s.io/apiextensions-apiserver v0.0.0-20220130045626-42a86de5afdd
	k8s.io/apimachinery => github.com/openshift/kubernetes/staging/src/k8s.io/apimachinery v0.0.0-20220130045626-42a86de5afdd
	k8s.io/apiserver => github.com/openshift/kubernetes/staging/src/k8s.io/apiserver v0.0.0-20220130045626-42a86de5afdd
	k8s.io/cli-runtime => github.com/openshift/kubernetes/staging/src/k8s.io/cli-runtime v0.0.0-20220130045626-42a86de5afdd
	k8s.io/client-go => github.com/openshift/kubernetes/staging/src/k8s.io/client-go v0.0.0-20220130045626-42a86de5afdd
	k8s.io/cloud-provider => github.com/openshift/kubernetes/staging/src/k8s.io/cloud-provider v0.0.0-20220130045626-42a86de5afdd
	k8s.io/cluster-bootstrap => github.com/openshift/kubernetes/staging/src/k8s.io/cluster-bootstrap v0.0.0-20220130045626-42a86de5afdd
	k8s.io/code-generator => github.com/openshift/kubernetes/staging/src/k8s.io/code-generator v0.0.0-20220130045626-42a86de5afdd
	k8s.io/component-base => github.com/openshift/kubernetes/staging/src/k8s.io/component-base v0.0.0-20220130045626-42a86de5afdd
	k8s.io/component-helpers => github.com/openshift/kubernetes/staging/src/k8s.io/component-helpers v0.0.0-20220130045626-42a86de5afdd
	k8s.io/controller-manager => github.com/openshift/kubernetes/staging/src/k8s.io/controller-manager v0.0.0-20220130045626-42a86de5afdd
	k8s.io/cri-api => github.com/openshift/kubernetes/staging/src/k8s.io/cri-api v0.0.0-20220130045626-42a86de5afdd
	k8s.io/csi-translation-lib => github.com/openshift/kubernetes/staging/src/k8s.io/csi-translation-lib v0.0.0-20220130045626-42a86de5afdd
	k8s.io/kube-aggregator => github.com/openshift/kubernetes/staging/src/k8s.io/kube-aggregator v0.0.0-20220130045626-42a86de5afdd
	k8s.io/kube-controller-manager => github.com/openshift/kubernetes/staging/src/k8s.io/kube-controller-manager v0.0.0-20220130045626-42a86de5afdd
	k8s.io/kube-proxy => github.com/openshift/kubernetes/staging/src/k8s.io/kube-proxy v0.0.0-20220130045626-42a86de5afdd
	k8s.io/kube-scheduler => github.com/openshift/kubernetes/staging/src/k8s.io/kube-scheduler v0.0.0-20220130045626-42a86de5afdd
	k8s.io/kubectl => github.com/openshift/kubernetes/staging/src/k8s.io/kubectl v0.0.0-20220130045626-42a86de5afdd
	k8s.io/kubelet => github.com/openshift/kubernetes/staging/src/k8s.io/kubelet v0.0.0-20220130045626-42a86de5afdd
	k8s.io/kubernetes => github.com/openshift/kubernetes v1.22.2-0.20220130045626-42a86de5afdd
	k8s.io/legacy-cloud-providers => github.com/openshift/kubernetes/staging/src/k8s.io/legacy-cloud-providers v0.0.0-20220130045626-42a86de5afdd
	k8s.io/metrics => github.com/openshift/kubernetes/staging/src/k8s.io/metrics v0.0.0-20220130045626-42a86de5afdd
	k8s.io/mount-utils => github.com/openshift/kubernetes/staging/src/k8s.io/mount-utils v0.0.0-20220130045626-42a86de5afdd
	k8s.io/pod-security-admission => github.com/openshift/kubernetes/staging/src/k8s.io/pod-security-admission v0.0.0-20220130045626-42a86de5afdd
	k8s.io/sample-apiserver => github.com/openshift/kubernetes/staging/src/k8s.io/sample-apiserver v0.0.0-20220130045626-42a86de5afdd
	k8s.io/sample-cli-plugin => github.com/openshift/kubernetes/staging/src/k8s.io/sample-cli-plugin v0.0.0-20220130045626-42a86de5afdd
	k8s.io/sample-controller => github.com/openshift/kubernetes/staging/src/k8s.io/sample-controller v0.0.0-20220130045626-42a86de5afdd
)
