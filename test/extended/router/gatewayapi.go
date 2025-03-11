package router

import (
	"context"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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
			"referencegrants.gateway.networking.k8s.io",
		}
		errorMessage = "ValidatingAdmissionPolicy 'openshift-ingress-operator-gatewayapi-crd-admission' with binding 'openshift-ingress-operator-gatewayapi-crd-admission' denied request: Gateway API Custom Resource Definitions are managed by the Ingress Operator and may not be modified"
	)

	g.Describe("The Gateway API CRDs", func() {
		g.It("should already be installed", func() {
			crdClient := apiextensionsclientset.NewForConfigOrDie(oc.AdminConfig())
			g.By("Get and check the installed CRDs")
			for i := range crdNames {
				crd, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), crdNames[i], metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf("Found the Gateway API CRD named: %v", crd.Name)
			}
		})

		g.It("should not be deleted", func() {
			ctx := context.Background()
			crdClient := apiextensionsclientset.NewForConfigOrDie(oc.AdminConfig())
			g.By("Trying to delete the CRDs")
			for i := range crdNames {
				err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, crdNames[i], metav1.DeleteOptions{})
				o.Expect(err).To(o.HaveOccurred())
				e2e.Logf("Got error: %v", err.Error())
				o.Expect(err.Error()).To(o.ContainSubstring(errorMessage))
			}
		})

		g.It("should not be updated", func() {
			crdClient := apiextensionsclientset.NewForConfigOrDie(oc.AdminConfig())
			g.By("Get the CRDs firstly, add spec.names.shortNames then update CRD")
			for i := range crdNames {
				crd, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), crdNames[i], metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				// some CRDs have a shortName but some not, just trying to add one for all
				crd.Spec.Names.ShortNames = append(crd.Spec.Names.ShortNames, "fakename")
				_, err = crdClient.ApiextensionsV1().CustomResourceDefinitions().Update(context.Background(), crd, metav1.UpdateOptions{})
				o.Expect(err).To(o.HaveOccurred())
				e2e.Logf("Got error: %v", err.Error())
				o.Expect(err.Error()).To(o.ContainSubstring(errorMessage))
			}
		})

		g.It("should not be created", func() {
			crdClient := apiextensionsclientset.NewForConfigOrDie(oc.AdminConfig())
			fakeCRDName := "fakeroutes.gateway.networking.k8s.io"
			g.By("Creating new CRD in the group")
			fakeCRD := buildGWAPICRDFromName(fakeCRDName)
			e2e.Logf("The CRD to be created is: %v", fakeCRD)
			_, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), fakeCRD, metav1.CreateOptions{})
			o.Expect(err).To(o.HaveOccurred())
			e2e.Logf("Got error: %v", err.Error())
			o.Expect(err.Error()).To(o.ContainSubstring(errorMessage))
		})
	})

	g.Describe("The ValidatingAdmissionPolicy for Gateway API", func() {
		// Adding [Serial] since it could break Gateway API CRDs creation/deletion/update tests
		g.It("should be recreated after it is deleted [Serial]", func() {
			var (
				ctx        = context.Background()
				policyName = "openshift-ingress-operator-gatewayapi-crd-admission"
				interval   = 5 * time.Second
				// VAP should be recreated in 5 minutes
				timeout = 5 * time.Minute
			)

			g.By("The VAP should exist in cluster")
			_, err := oc.AdminKubeClient().AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, policyName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("Delete the VAP")
			err = oc.AdminKubeClient().AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(ctx, policyName, metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("The VAP should be recreated in allowed time")
			waitErr := wait.PollUntilContextTimeout(ctx, interval, timeout, false, func(ctx context.Context) (bool, error) {
				policy, err := oc.AdminKubeClient().AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, policyName, metav1.GetOptions{})
				if err != nil {
					e2e.Logf("Got error: %v, try again...", err.Error())
					return false, nil
				}
				if policy.Status.TypeChecking != nil {
					e2e.Logf("Found the VAP named %v", policy.Name)
					return true, nil
				}
				return false, nil
			})
			o.Expect(waitErr).NotTo(o.HaveOccurred())
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
