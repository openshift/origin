package usertags

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/stretchr/objx"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	azutil "github.com/openshift/origin/test/extended/util/azure"
)

var _ = g.Describe("[sig-installer][Feature:AzureUserTags] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("have user defined tags present on all the created resources", func() {
		ctx := context.Background()
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())

		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("check is platform type Azure in Infrastructure cluster resource")
		cfgClient := dc.Resource(schema.GroupVersionResource{Group: "config.openshift.io", Resource: "infrastructures", Version: "v1"})
		infraobj, err := cfgClient.Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(infraobj).NotTo(o.BeNil(),
			fmt.Sprintf("failed to fetch infrastructure/cluster resource: %+v", err))

		// if platform type is not Azure skip tests.
		azutil.SkipUnlessPlatformAzure(objx.Map(infraobj.UnstructuredContent()))

		// get InfrastructureName as defined in infrastructure/cluster resource.
		infraName := azutil.GetInfrastructureName(infraobj.UnstructuredContent())
		o.Expect(infraName).NotTo(o.BeEmpty())
		resourceGroupName := fmt.Sprintf("%s-rg", infraName)

		// get list of tags present infrastructure/cluster resource as defined
		// by the user during cluster creation.
		infraTagList := azutil.GetInfraResourceTags(infraobj.UnstructuredContent())
		o.Expect(infraTagList).NotTo(o.BeEmpty())
		infraTagList[fmt.Sprintf("kubernetes.io_cluster.%s", infraName)] = "owned"

		// get list of resources with the tags present on it in a resourcegroup
		// along with the resourcegroup tags.
		resourceTagList, err := azutil.ListResources(ctx, resourceGroupName)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(resourceTagList).NotTo(o.BeEmpty())

		// check all the tags present in infrastructure/cluster resource is
		// present in all the resources supporting tags.
		for _, resourceTagSet := range resourceTagList {
			for k, v := range infraTagList {
				// skip checking resources not created by OpenShift
				// core operators or installer
				if resourceTagSet.Type == "Microsoft.Network/publicIPAddresses" {
					// skip resource created by cluster-cloud-controller-manager-operator
					if _, exist := resourceTagSet.Tags["k8s-azure-service"]; exist {
						continue
					}
				}
				o.Expect(resourceTagSet.Tags).Should(
					o.HaveKeyWithValue(k, v),
					fmt.Sprintf("%s/%s does not match required tagset", resourceTagSet.Type, resourceTagSet.Name))
			}
		}
	})
})
