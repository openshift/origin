package apiserver

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/openshift/origin/test/extended/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"

	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-api-machinery] API", func() {
	defer g.GinkgoRecover()

	var resourceLists []*metav1.APIResourceList
	var groups []*metav1.APIGroup

	g.BeforeEach(func() {
		kubeConfig, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeConfig)
		o.Expect(err).NotTo(o.HaveOccurred())
		groups, resourceLists, err = discoveryClient.ServerGroupsAndResources()
		o.Expect(err).NotTo(o.HaveOccurred())
		//scheme = runtime.NewScheme()
		//err = api.Install(scheme)
		//o.Expect(err).NotTo(o.HaveOccurred())
		//err = api.InstallKube(scheme)
		//o.Expect(err).NotTo(o.HaveOccurred())
		//restMapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))
		//restMapper.ResourceFor()
	})

	g.It("should specify compatibility level 4 if exposed and internal.", func() {
		for _, g := range resourceLists {
			for _, r := range g.APIResources {
				gvk := schema.FromAPIVersionAndKind(g.GroupVersion, r.Kind)
				obj, err := scheme.Scheme.New(gvk)
				if err != nil {
					e2e.Logf("%v %v: %v", gvk.GroupVersion(), gvk.Kind, err)
				} else {
					e2e.Logf("%v %v", gvk.GroupVersion(), gvk.Kind)
				}
				//o.Expect(err).NotTo(o.HaveOccurred(), "API should be installed into scheme")
				a, ok := obj.(interface{ Internal() bool })
				if ok && a.Internal() {
					b, ok := obj.(interface{ CompatibilityLevel() int })
					o.Expect(ok).To(o.BeTrue(), "exposed internal type must specify a compatibility level")
					o.Expect(b.CompatibilityLevel()).To(o.Equal(4), "exposed internal type must specify a compatibility level 4")
				}
			}
		}
		//g.Fail("YES")
	})
})
