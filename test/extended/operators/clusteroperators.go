package operators

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	s "github.com/onsi/gomega/gstruct"
	t "github.com/onsi/gomega/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kube-openapi/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	config "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Platform] ClusterOperators", func() {
	defer g.GinkgoRecover()

	var clusterOperators []config.ClusterOperator
	whitelistNoNamespace := sets.NewString(
		"cloud-credential",
		"image-registry",
		"machine-api",
		"marketplace",
		"network",
		"operator-lifecycle-manager",
		"operator-lifecycle-manager-catalog",
		"support",
	)
	whitelistNoOperatorConfig := sets.NewString(
		"cloud-credential",
		"cluster-autoscaler",
		"machine-api",
		"machine-config",
		"marketplace",
		"network",
		"operator-lifecycle-manager",
		"operator-lifecycle-manager-catalog",
		"support",
	)

	g.BeforeEach(func() {
		kubeConfig, err := e2e.LoadConfig()
		o.Expect(err).ToNot(o.HaveOccurred())
		configClient, err := configclient.NewForConfig(kubeConfig)
		o.Expect(err).ToNot(o.HaveOccurred())
		clusterOperatorsList, err := configClient.ClusterOperators().List(metav1.ListOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		clusterOperators = clusterOperatorsList.Items
	})

	g.Context("should define", func() {
		g.Specify("at least one namespace in their lists of related objects", func() {
			for _, clusterOperator := range clusterOperators {
				if !whitelistNoNamespace.Has(clusterOperator.Name) {
					o.Expect(clusterOperator.Status.RelatedObjects).To(o.ContainElement(isNamespace()), "ClusterOperator: %s", clusterOperator.Name)
				}
			}
		})
		g.Specify("at least one related object that is not a namespace", func() {
			for _, clusterOperator := range clusterOperators {
				if !whitelistNoOperatorConfig.Has(clusterOperator.Name) {
					o.Expect(clusterOperator.Status.RelatedObjects).To(o.ContainElement(o.Not(isNamespace())), "ClusterOperator: %s", clusterOperator.Name)
				}
			}
		})

	})

	g.Describe("fields validation", func() {
		var (
			oc                               = exutil.NewCLI("cluster-basic-auth", exutil.KubeConfigPath())
			openshiftApiServerOperatorClient = oc.AdminOperatorClient().OperatorV1().OpenShiftAPIServers()
		)
		defer g.GinkgoRecover()

		g.It("managementState [Serial][Disruptive]", func() {
			defer func() {
				oc.AdminOperatorClient().OperatorV1().OpenShiftAPIServers().Patch("cluster", types.JSONPatchType, []byte(`[{"op": "replace", "path": "/spec/managementState", "value": "Managed"}]`))
			}()
			g.By(fmt.Sprintf("update managementState with Unmanaged"))
			_, err := openshiftApiServerOperatorClient.Patch("cluster", types.JSONPatchType, []byte(`[{"op": "replace", "path": "/spec/managementState", "value": "Unmanaged"}]`))
			o.Expect(err).NotTo(o.HaveOccurred())

			err = oc.AdminKubeClient().CoreV1().Services("openshift-apiserver").Delete("api", nil)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("make sure no service being created"))
			err = wait.PollImmediate(200*time.Millisecond, 5*time.Minute, func() (bool, error) {
				svcs, err := oc.AdminKubeClient().CoreV1().Services("openshift-apiserver").List(metav1.ListOptions{})
				if err != nil {
					return false, err
				}
				return len(svcs.Items) == 0, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("update managementState with Removed"))
			_, err = openshiftApiServerOperatorClient.Patch("cluster", types.JSONPatchType, []byte(`[{"op": "replace", "path": "/spec/managementState", "value": "Removed"}]`))
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("make sure that all pods being deleted"))
			err = wait.PollImmediate(200*time.Millisecond, 5*time.Minute, func() (bool, error) {
				pods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-apiserver").List(metav1.ListOptions{})
				if err != nil {
					return false, err
				}
				return len(pods.Items) == 0, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})

func isNamespace() t.GomegaMatcher {
	return s.MatchFields(s.IgnoreExtras|s.IgnoreMissing, s.Fields{
		"Resource": o.Equal("namespaces"),
		"Group":    o.Equal(""),
	})
}
