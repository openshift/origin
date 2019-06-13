package cli

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	appsv1 "github.com/openshift/api/apps/v1"
	"github.com/openshift/library-go/pkg/apps/appsutil"

	exutil "github.com/openshift/origin/test/extended/util"
)

const deploymentRunTimeout = 5 * time.Minute
const deploymentChangeTimeout = 30 * time.Second

var _ = g.Describe("[cli][Slow] scale", func() {
	defer g.GinkgoRecover()

	var oc *exutil.CLI

	oc = exutil.NewCLI("cli-deployment", exutil.KubeConfigPath())

	var (
		simpleDeploymentFixture = exutil.FixturePath("testdata", "deployments", "deployment-simple.yaml")
	)

	g.Describe("scale [Conformance]", func() {
		dcName := "deployment-simple"

		g.It("scale DC will update replicas", func() {
			namespace := oc.Namespace()

			dc := exutil.ReadFixtureOrFail(simpleDeploymentFixture).(*appsv1.DeploymentConfig)
			o.Expect(dc.Name).To(o.Equal(dcName))

			dc.Spec.Replicas = 1
			dc.Spec.MinReadySeconds = 60
			dc, err := oc.AppsClient().AppsV1().DeploymentConfigs(namespace).Create(dc)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Name).To(o.Equal(dcName))

			g.By("during the deployer pod running scale up the DC")
			g.By("waiting for RC to be created")
			dc, err = waitForDCModification(oc, namespace, dc.Name, deploymentRunTimeout,
				dc.GetResourceVersion(), func(config *appsv1.DeploymentConfig) (bool, error) {
					cond := appsutil.GetDeploymentCondition(config.Status, appsv1.DeploymentProgressing)
					if cond != nil && cond.Reason == appsutil.NewReplicationControllerReason {
						return true, nil
					}
					return false, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(dc.Status.LatestVersion).To(o.BeEquivalentTo(1))

			g.By("waiting for deployer pod to be running")
			_, err = waitForRCModification(oc, namespace, appsutil.LatestDeploymentNameForConfigAndVersion(dc.Name, dc.Status.LatestVersion),
				deploymentRunTimeout,
				"", func(currentRC *corev1.ReplicationController) (bool, error) {
					if appsutil.DeploymentStatusFor(currentRC) == appsv1.DeploymentStatusRunning {
						return true, nil
					}
					return false, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("scale up the deployment")
			_, err = oc.Run("scale").Args("dc/deployment-simple", "--replicas=2").Output()

			g.By(fmt.Sprintf("by checking that the deployment config has the correct replicas"))
			_, err = waitForRCModification(oc, namespace, appsutil.LatestDeploymentNameForConfigAndVersion(dc.Name, dc.Status.LatestVersion),
				deploymentRunTimeout,
				"", func(currentRC *corev1.ReplicationController) (bool, error) {
					if appsutil.DeploymentStatusFor(currentRC) == appsv1.DeploymentStatusComplete {
						return true, nil
					}
					return false, nil
				})
			o.Expect(err).NotTo(o.HaveOccurred())
			err = wait.PollImmediate(500*time.Millisecond, time.Minute, func() (bool, error) {
				dc, err := oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Get(dc.Name, metav1.GetOptions{})
				if err != nil {
					return false, nil
				}
				return dc.Spec.Replicas == 2, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("scale up the deployment when deploy complete")
			_, err = oc.Run("scale").Args("dc/deployment-simple", "--replicas=5").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("ensuring the scale up of the deployment happens")
			err = wait.PollImmediate(500*time.Millisecond, time.Minute, func() (bool, error) {
				dc, err := oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Get(dc.Name, metav1.GetOptions{})
				if err != nil {
					return false, nil
				}
				return dc.Spec.Replicas == 5, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("scale down the deployment when deploy complete")
			_, err = oc.Run("scale").Args("dc/deployment-simple", "--replicas=3").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("ensuring the scale up of the deployment happens")
			wait.PollImmediate(100*time.Millisecond, 10*time.Second, func() (bool, error) {
				dc, err := oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Get(dc.Name, metav1.GetOptions{})
				if err != nil {
					return false, nil
				}
				return dc.Spec.Replicas == 3, nil
			})
		})
	})
})
