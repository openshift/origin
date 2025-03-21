package router

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
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
			crdClient := apiextensionsclientset.NewForConfigOrDie(oc.AdminConfig())
			g.By("Trying to delete the CRDs")
			for i := range crdNames {
				err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), crdNames[i], metav1.DeleteOptions{})
				o.Expect(err).To(o.HaveOccurred())
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
				o.Expect(err.Error()).To(o.ContainSubstring(errorMessage))
			}
		})

		g.It("should not be created", func() {
			crdClient := apiextensionsclientset.NewForConfigOrDie(oc.AdminConfig())
			fakeCRDName := "fakeroutes.gateway.networking.k8s.io"
			g.By("Creating new CRD in the group")
			fakeCRD := buildGWAPICRDFromName(fakeCRDName)
			_, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), fakeCRD, metav1.CreateOptions{})
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.ContainSubstring(errorMessage))
		})
	})
})

// Adding [Serial] since this test needs to scale down CVO and remove VAP, it could break other Gateway API CRDs tests
var _ = g.Describe("[sig-network][OCPFeatureGate:GatewayAPI][Serial][Feature:Router][apigroup:gateway.networking.k8s.io]", func() {
	defer g.GinkgoRecover()
	var (
		oc       = exutil.NewCLIWithPodSecurityLevel("gateway-api", admissionapi.LevelBaseline)
		crdNames = []string{
			"gatewayclasses.gateway.networking.k8s.io",
			"gateways.gateway.networking.k8s.io",
			"httproutes.gateway.networking.k8s.io",
			"referencegrants.gateway.networking.k8s.io",
		}
	)

	g.Describe("The Gateway API CRDs", func() {
		g.It("can be deleted once VAP is removed", func() {
			var (
				policyName   = "openshift-ingress-operator-gatewayapi-crd-admission"
				namespace    = "openshift-cluster-version"
				deployName   = "cluster-version-operator"
				defaultScale = 1
				newScale     = 0
			)
			crdClient := apiextensionsclientset.NewForConfigOrDie(oc.AdminConfig())

			g.By("Scale down the CVO")
			podsLabel, err := labels.Parse("k8s-app=cluster-version-operator")
			o.Expect(err).To(o.BeNil(), "Expected to parse pods label selector without error")
			defer func() {
				_, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Patch(context.Background(), deployName, types.StrategicMergePatchType, []byte(fmt.Sprintf(`{"spec": {"replicas": %d}}`, defaultScale)), metav1.PatchOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = exutil.WaitForPods(oc.AdminKubeClient().CoreV1().Pods(namespace), podsLabel, exutil.CheckPodIsReady, 1, 10*time.Second)
				o.Expect(err).NotTo(o.HaveOccurred())
			}()

			_, err = oc.AdminKubeClient().AppsV1().Deployments(namespace).Patch(context.Background(), deployName, types.StrategicMergePatchType, []byte(fmt.Sprintf(`{"spec": {"replicas": %d}}`, newScale)), metav1.PatchOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = exutil.WaitForPods(oc.AdminKubeClient().CoreV1().Pods(namespace), podsLabel, exutil.CheckPodIsRunning, 0, 10*time.Second)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Deleting the VAP should succeed")
			err = oc.AdminKubeClient().AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(context.Background(), policyName, metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Trying to delete the CRDs should succeed")
			for i := range crdNames {
				// API server needs some time to react to the VAP removal
				waitErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 10*time.Second, true, func(ctx context.Context) (bool, error) {
					err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), crdNames[i], metav1.DeleteOptions{})
					if err != nil {
						e2e.Logf("Got error: %v, try again later...", err.Error())
						return false, nil
					}
					return true, nil
				})
				o.Expect(waitErr).NotTo(o.HaveOccurred())
			}

			g.By("The CRDs should be recreated by CIO")
			for i := range crdNames {
				waitErr := wait.PollUntilContextTimeout(context.Background(), 1*time.Second, 10*time.Second, true, func(ctx context.Context) (bool, error) {
					crd, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), crdNames[i], metav1.GetOptions{})
					if err != nil {
						e2e.Logf("Got error: %v, try again later...", err.Error())
						return false, nil
					}
					if crd.Name == crdNames[i] {
						e2e.Logf("Found the Gateway API CRD named %v", crd.Name)
						return true, nil
					}
					return false, nil
				})
				o.Expect(waitErr).NotTo(o.HaveOccurred())
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
