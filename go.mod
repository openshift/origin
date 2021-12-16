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
	github.com/openshift/api v0.0.0-20211028023115-7224b732cc14
	github.com/openshift/apiserver-library-go v0.0.0-20211105091019-06e87e7030eb
	github.com/openshift/build-machinery-go v0.0.0-20210806203541-4ea9b6da3a37
	github.com/openshift/client-go v0.0.0-20210831095141-e19a065e79f7
	github.com/openshift/library-go v0.0.0-20211014100835-efbd9a7e5841
	github.com/pborman/uuid v1.2.0
	github.com/pquerna/cachecontrol v0.0.0-20201205024021-ac21108117ac // indirect
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.26.0
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.0
	github.com/stretchr/objx v0.2.0
	github.com/stretchr/testify v1.7.0
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	go.etcd.io/etcd v0.5.0-alpha.5.0.20200910180754-dd1b699fc489
	go.etcd.io/etcd/api/v3 v3.5.0
	go.etcd.io/etcd/client/v3 v3.5.0
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83
	golang.org/x/net v0.0.0-20210520170846-37e1c6afe023
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	google.golang.org/grpc v1.38.0
	gopkg.in/ldap.v2 v2.5.1
	gopkg.in/src-d/go-git.v4 v4.13.1
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.22.2
	k8s.io/apiextensions-apiserver v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/apiserver v0.22.2
	k8s.io/cli-runtime v0.22.2
	k8s.io/client-go v0.22.2
	k8s.io/component-base v0.22.2
	k8s.io/component-helpers v0.0.0
	k8s.io/klog/v2 v2.9.0
	k8s.io/kube-aggregator v0.22.2
	k8s.io/kube-openapi v0.0.0-20210421082810-95288971da7e
	k8s.io/kubectl v0.22.2
	k8s.io/kubelet v0.22.2
	k8s.io/kubernetes v1.22.1
	k8s.io/legacy-cloud-providers v0.22.2
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/google/cadvisor => github.com/openshift/google-cadvisor v0.33.2-0.20210825140152-3e7875ab2744
	github.com/onsi/ginkgo => github.com/openshift/onsi-ginkgo v4.7.0-origin.0+incompatible
	k8s.io/api => github.com/openshift/kubernetes/staging/src/k8s.io/api v0.0.0-20211214021038-e790d7fa218b
	k8s.io/apiextensions-apiserver => github.com/openshift/kubernetes/staging/src/k8s.io/apiextensions-apiserver v0.0.0-20211214021038-e790d7fa218b
	k8s.io/apimachinery => github.com/openshift/kubernetes/staging/src/k8s.io/apimachinery v0.0.0-20211214021038-e790d7fa218b
	k8s.io/apiserver => github.com/openshift/kubernetes/staging/src/k8s.io/apiserver v0.0.0-20211214021038-e790d7fa218b
	k8s.io/cli-runtime => github.com/openshift/kubernetes/staging/src/k8s.io/cli-runtime v0.0.0-20211214021038-e790d7fa218b
	k8s.io/client-go => github.com/openshift/kubernetes/staging/src/k8s.io/client-go v0.0.0-20211214021038-e790d7fa218b
	k8s.io/cloud-provider => github.com/openshift/kubernetes/staging/src/k8s.io/cloud-provider v0.0.0-20211214021038-e790d7fa218b
	k8s.io/cluster-bootstrap => github.com/openshift/kubernetes/staging/src/k8s.io/cluster-bootstrap v0.0.0-20211214021038-e790d7fa218b
	k8s.io/code-generator => github.com/openshift/kubernetes/staging/src/k8s.io/code-generator v0.0.0-20211214021038-e790d7fa218b
	k8s.io/component-base => github.com/openshift/kubernetes/staging/src/k8s.io/component-base v0.0.0-20211214021038-e790d7fa218b
	k8s.io/component-helpers => github.com/openshift/kubernetes/staging/src/k8s.io/component-helpers v0.0.0-20211214021038-e790d7fa218b
	k8s.io/controller-manager => github.com/openshift/kubernetes/staging/src/k8s.io/controller-manager v0.0.0-20211214021038-e790d7fa218b
	k8s.io/cri-api => github.com/openshift/kubernetes/staging/src/k8s.io/cri-api v0.0.0-20211214021038-e790d7fa218b
	k8s.io/csi-translation-lib => github.com/openshift/kubernetes/staging/src/k8s.io/csi-translation-lib v0.0.0-20211214021038-e790d7fa218b
	k8s.io/kube-aggregator => github.com/openshift/kubernetes/staging/src/k8s.io/kube-aggregator v0.0.0-20211214021038-e790d7fa218b
	k8s.io/kube-controller-manager => github.com/openshift/kubernetes/staging/src/k8s.io/kube-controller-manager v0.0.0-20211214021038-e790d7fa218b
	k8s.io/kube-proxy => github.com/openshift/kubernetes/staging/src/k8s.io/kube-proxy v0.0.0-20211214021038-e790d7fa218b
	k8s.io/kube-scheduler => github.com/openshift/kubernetes/staging/src/k8s.io/kube-scheduler v0.0.0-20211214021038-e790d7fa218b
	k8s.io/kubectl => github.com/openshift/kubernetes/staging/src/k8s.io/kubectl v0.0.0-20211214021038-e790d7fa218b
	k8s.io/kubelet => github.com/openshift/kubernetes/staging/src/k8s.io/kubelet v0.0.0-20211214021038-e790d7fa218b
	k8s.io/kubernetes => github.com/openshift/kubernetes v1.22.2-0.20211214021038-e790d7fa218b
	k8s.io/legacy-cloud-providers => github.com/openshift/kubernetes/staging/src/k8s.io/legacy-cloud-providers v0.0.0-20211214021038-e790d7fa218b
	k8s.io/metrics => github.com/openshift/kubernetes/staging/src/k8s.io/metrics v0.0.0-20211214021038-e790d7fa218b
	k8s.io/mount-utils => github.com/openshift/kubernetes/staging/src/k8s.io/mount-utils v0.0.0-20211214021038-e790d7fa218b
	k8s.io/pod-security-admission => github.com/openshift/kubernetes/staging/src/k8s.io/pod-security-admission v0.0.0-20211214021038-e790d7fa218b
	k8s.io/sample-apiserver => github.com/openshift/kubernetes/staging/src/k8s.io/sample-apiserver v0.0.0-20211214021038-e790d7fa218b
	k8s.io/sample-cli-plugin => github.com/openshift/kubernetes/staging/src/k8s.io/sample-cli-plugin v0.0.0-20211214021038-e790d7fa218b
	k8s.io/sample-controller => github.com/openshift/kubernetes/staging/src/k8s.io/sample-controller v0.0.0-20211214021038-e790d7fa218b
)
