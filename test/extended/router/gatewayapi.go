package router

import (
	"context"
	"slices"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = g.Describe("[sig-network][OCPFeatureGate:GatewayAPI][Feature:Router][apigroup:gateway.networking.k8s.io]", func() {
	defer g.GinkgoRecover()
	var (
		oc       = exutil.NewCLIWithPodSecurityLevel("gateway-api", admissionapi.LevelBaseline)
		crdNames = []string{
			"gatewayclasses.gateway.networking.k8s.io",
			"gateways.gateway.networking.k8s.io",
			"httproutes.gateway.networking.k8s.io",
			"grpcroutes.gateway.networking.k8s.io",
			"referencegrants.gateway.networking.k8s.io",
		}
		errorMessage = "ValidatingAdmissionPolicy 'openshift-ingress-operator-gatewayapi-crd-admission' with binding 'openshift-ingress-operator-gatewayapi-crd-admission' denied request: Gateway API Custom Resource Definitions are managed by the Ingress Operator and may not be modified"
	)

	g.Describe("Verify Gateway API CRDs", func() {
		g.It("and ensure required CRDs should already be installed", func() {
			g.By("Get and check the installed CRDs")
			for i := range crdNames {
				crd, err := oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), crdNames[i], metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf("Found the Gateway API CRD named: %v", crd.Name)
			}
		})

		g.It("and ensure existing CRDs can not be deleted", func() {
			g.By("Try to delete the CRDs and fail")
			for i := range crdNames {
				err := oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), crdNames[i], metav1.DeleteOptions{})
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(err.Error()).To(o.ContainSubstring(errorMessage))
			}
		})

		g.It("and ensure existing CRDs can not be updated", func() {
			g.By("Get the CRDs firstly, add spec.names.shortNames then update CRD")
			for i := range crdNames {
				crd, err := oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), crdNames[i], metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				// some CRDs have a shortName but some not, just trying to add one for all
				crd.Spec.Names.ShortNames = append(crd.Spec.Names.ShortNames, "fakename")
				_, err = oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Update(context.Background(), crd, metav1.UpdateOptions{})
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(err.Error()).To(o.ContainSubstring(errorMessage))
			}
		})

		g.It("and ensure CRD of standard group can not be created", func() {
			fakeCRDName := "fakeroutes.gateway.networking.k8s.io"
			g.By("Try to create CRD of standard group and fail")
			fakeCRD := buildGWAPICRDFromName(fakeCRDName)
			_, err := oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), fakeCRD, metav1.CreateOptions{})
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.ContainSubstring(errorMessage))
		})

		g.It("and ensure CRD of experimental group is not installed", func() {
			g.By("Ensure no CRD of experimental group is installed")
			crdList, err := oc.AdminApiextensionsClient().ApiextensionsV1().CustomResourceDefinitions().List(context.Background(), metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, crd := range crdList.Items {
				if crd.Spec.Group == "gateway.networking.x-k8s.io" {
					e2e.Failf("Found unexpected CRD named: %v", crd.Name)
				}
				// Check standard group GWAPI CRDs to ensure they are not in the experimental channel
				if slices.Contains(crdNames, crd.Name) {
					if channel, ok := crd.Annotations["gateway.networking.k8s.io/channel"]; ok && channel == "experimental" {
						e2e.Failf("Found experimental channel CRD: %v (expected standard channel)", crd.Name)
					}
				}
			}
		})
	})
})

// buildGWAPICRDFromName initializes the fake GatewayAPI CRD deducing most of its required fields from the given name.
func buildGWAPICRDFromName(name string) *apiextensionsv1.CustomResourceDefinition {
	var (
		plural   = strings.Split(name, ".")[0]
		group, _ = strings.CutPrefix(name, plural+".")
		// removing trailing "s"
		singular = plural[0 : len(plural)-1]
		kind     = strings.Title(singular)
	)

	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: plural + "." + group,
			Annotations: map[string]string{
				"api-approved.kubernetes.io": "https://github.com/kubernetes-sigs/gateway-api/pull/2466",
			},
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Singular: singular,
				Plural:   plural,
				Kind:     kind,
			},
			Scope: apiextensionsv1.ClusterScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Storage: true,
					Served:  true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
						},
					},
				},
				{
					Name:    "v1beta1",
					Storage: false,
					Served:  true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
						},
					},
				},
			},
		},
	}
}
