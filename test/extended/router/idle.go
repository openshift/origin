package router

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	unidlingapi "github.com/openshift/api/unidling/v1alpha1"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("[sig-network-edge][Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()

	var (
		configPath = exutil.FixturePath("testdata", "router", "router-idle.yaml")
		oc         = exutil.NewCLI("router-idling")
	)

	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed {
			exutil.DumpPodLogsStartingWithInNamespace("router", "openshift-ingress", oc.AsAdmin())
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should be able to connect to a service that is idled because a GET on the route will unidle it", func() {
			// timeout for kube GET polling operations
			timeout := 3 * time.Minute

			// timout for GET requests
			urlTimeout := 1 * time.Minute

			g.By(fmt.Sprintf("creating test fixture from a config file %q", configPath))
			err := oc.Run("new-app").Args("-f", configPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			urlTester := url.NewTester(oc.AdminKubeClient(), oc.Namespace()).WithErrorPassthrough(true)
			defer urlTester.Close()
			hostname := getHostnameForRoute(oc, "idle-test")
			urlTester.Within(urlTimeout, url.Expect("GET", "http://"+hostname).Through(hostname).HasStatusCode(200))

			g.By("Idling the service")
			_, err = oc.Run("idle").Args("idle-test").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			var annotations map[string]string

			g.By("Fetching the endpoints and checking the idle annotations are present")
			err = wait.PollImmediate(time.Second, timeout, func() (bool, error) {
				endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), "idle-test", metav1.GetOptions{})
				if err != nil {
					e2e.Logf("Error getting endpoints: %v", err)
					return false, nil
				}
				annotations = endpoints.Annotations
				_, idledAt := annotations[unidlingapi.IdledAtAnnotation]
				_, unidleTarget := annotations[unidlingapi.UnidleTargetAnnotation]
				return idledAt && unidleTarget, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			checkIdleAnnotationValues(annotations)

			g.By("Fetching the service and checking the idle annotations are present")
			err = wait.PollImmediate(time.Second, timeout, func() (bool, error) {
				service, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(context.Background(), "idle-test", metav1.GetOptions{})
				if err != nil {
					e2e.Logf("Error getting service: %v", err)
					return false, nil
				}
				annotations = service.Annotations
				_, idledAt := annotations[unidlingapi.IdledAtAnnotation]
				_, unidleTarget := annotations[unidlingapi.UnidleTargetAnnotation]
				return idledAt && unidleTarget, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			checkIdleAnnotationValues(annotations)

			g.By("Unidling up the service by making a GET request on the route")
			urlTester.Within(urlTimeout, url.Expect("GET", "http://"+hostname).Through(hostname).HasStatusCode(200))

			g.By("Validating that the idle annotations have been removed from the endpoints")
			err = wait.PollImmediate(time.Second, timeout, func() (bool, error) {
				endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), "idle-test", metav1.GetOptions{})
				if err != nil {
					e2e.Logf("Error getting endpoints: %v", err)
					return false, nil
				}
				_, idledAt := endpoints.Annotations[unidlingapi.IdledAtAnnotation]
				_, unidleTarget := endpoints.Annotations[unidlingapi.UnidleTargetAnnotation]
				return !idledAt && !unidleTarget, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Validating that the idle annotations have been removed from the service")
			err = wait.PollImmediate(time.Second, timeout, func() (bool, error) {
				service, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(context.Background(), "idle-test", metav1.GetOptions{})
				if err != nil {
					e2e.Logf("Error getting service: %v", err)
					return false, nil
				}
				_, idledAt := service.Annotations[unidlingapi.IdledAtAnnotation]
				_, unidleTarget := service.Annotations[unidlingapi.UnidleTargetAnnotation]
				return !idledAt && !unidleTarget, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})

func checkIdleAnnotationValues(annotations map[string]string) {
	o.Expect(annotations).To(o.HaveKey(unidlingapi.IdledAtAnnotation))
	o.Expect(annotations).To(o.HaveKey(unidlingapi.UnidleTargetAnnotation))

	idledAtAnnotation := annotations[unidlingapi.IdledAtAnnotation]
	idledAtTime, err := time.Parse(time.RFC3339, idledAtAnnotation)
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(idledAtTime).To(o.BeTemporally("~", time.Now(), 5*time.Minute))

	g.By("Checking the idle targets")
	unidleTargetAnnotation := annotations[unidlingapi.UnidleTargetAnnotation]
	var unidleTargets []unidlingapi.RecordedScaleReference
	err = json.Unmarshal([]byte(unidleTargetAnnotation), &unidleTargets)
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(unidleTargets).To(o.Equal([]unidlingapi.RecordedScaleReference{
		{
			Replicas: 1,
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind:  "Deployment",
				Group: "apps",
				Name:  "idle-test",
			},
		},
	}))
}
