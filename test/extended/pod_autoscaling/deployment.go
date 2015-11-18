package pod_autoscaling

import (
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/kubernetes/pkg/api"
	exapi "k8s.io/kubernetes/pkg/apis/extensions"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/kubernetes/pkg/util/wait"
)

var _ = g.Describe("Horizontal Pod Autoscaling: DeploymentConfigs", func() {
	defer g.GinkgoRecover()

	var (
		oc                      = exutil.NewCLI("hpa-tests", exutil.KubeConfigPath())
		deploymentConfigFixture = exutil.FixturePath("fixtures", "test-basic-deploymentconfig.yaml")
		dcName                  = "frontend"
	)

	g.JustBeforeEach(func() {
		g.By("creating the DeploymentConfig")
		err := oc.Run("create").Args("-f", deploymentConfigFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should scale the deployment when it is complete", func() {
		err := oc.Run("deploy").Args(dcName, "--latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		rcClient := oc.KubeREST().ReplicationControllers(oc.Namespace())
		err = exutil.WaitForADeployment(rcClient, dcName, exutil.CheckDeploymentCompletedFunc, exutil.CheckDeploymentFailedFunc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating a new HPA targeted at 80% CPU consumption")
		scaler := exapi.HorizontalPodAutoscaler{
			ObjectMeta: kapi.ObjectMeta{Name: "frontend-scaler"},
			Spec: exapi.HorizontalPodAutoscalerSpec{
				ScaleRef: exapi.SubresourceReference{
					Kind:        "DeploymentConfig",
					Name:        dcName,
					APIVersion:  "v1",
					Subresource: "scale",
				},
				MaxReplicas:    6,
				CPUUtilization: &exapi.CPUTargetUtilization{TargetPercentage: 80},
			},
		}
		_, err = oc.KubeREST().Extensions().HorizontalPodAutoscalers(oc.Namespace()).Create(&scaler)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForRCToBeScaled(rcClient, deployutil.DeploymentNameForConfigVersion(dcName, 1), 1)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should only scale the most recent deployment", func() {
		err := oc.Run("deploy").Args(dcName, "--latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		rcClient := oc.KubeREST().ReplicationControllers(oc.Namespace())
		err = exutil.WaitForADeployment(rcClient, dcName, exutil.CheckDeploymentCompletedFunc, exutil.CheckDeploymentFailedFunc)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("deploy").Args(dcName, "--latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForADeployment(rcClient, dcName, exutil.CheckDeploymentCompletedFunc, exutil.CheckDeploymentFailedFunc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating a new HPA targeted at 80% CPU consumption")
		scaler := exapi.HorizontalPodAutoscaler{
			ObjectMeta: kapi.ObjectMeta{Name: "frontend-scaler"},
			Spec: exapi.HorizontalPodAutoscalerSpec{
				ScaleRef: exapi.SubresourceReference{
					Kind:        "DeploymentConfig",
					Name:        dcName,
					APIVersion:  "v1",
					Subresource: "scale",
				},
				MaxReplicas:    6,
				CPUUtilization: &exapi.CPUTargetUtilization{TargetPercentage: 80},
			},
		}
		_, err = oc.KubeREST().Extensions().HorizontalPodAutoscalers(oc.Namespace()).Create(&scaler)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForRCToBeScaled(rcClient, deployutil.DeploymentNameForConfigVersion(dcName, 1), 0)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForRCToBeScaled(rcClient, deployutil.DeploymentNameForConfigVersion(dcName, 2), 1)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should not scale a failed or incomplete deployment", func() {
		err := oc.Run("deploy").Args(dcName, "--latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		rcClient := oc.KubeREST().ReplicationControllers(oc.Namespace())
		err = exutil.WaitForADeployment(rcClient, dcName, exutil.CheckDeploymentCompletedFunc, exutil.CheckDeploymentFailedFunc)
		o.Expect(err).NotTo(o.HaveOccurred())

		// update the deployment to mark it as failed
		rc, err := rcClient.Get(deployutil.DeploymentNameForConfigVersion(dcName, 1))
		o.Expect(err).NotTo(o.HaveOccurred())
		rc.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusFailed)
		_, err = rcClient.Update(rc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating a new HPA targeted at 80% CPU consumption")
		scaler := exapi.HorizontalPodAutoscaler{
			ObjectMeta: kapi.ObjectMeta{Name: "frontend-scaler"},
			Spec: exapi.HorizontalPodAutoscalerSpec{
				ScaleRef: exapi.SubresourceReference{
					Kind:        "DeploymentConfig",
					Name:        dcName,
					APIVersion:  "v1",
					Subresource: "scale",
				},
				MaxReplicas:    6,
				CPUUtilization: &exapi.CPUTargetUtilization{TargetPercentage: 80},
			},
		}
		_, err = oc.KubeREST().Extensions().HorizontalPodAutoscalers(oc.Namespace()).Create(&scaler)
		o.Expect(err).NotTo(o.HaveOccurred())

		// this needs to be "wait till RC *could* be scaled (check last scale time?)
		err = waitForRCToBeScaled(rcClient, deployutil.DeploymentNameForConfigVersion(dcName, 1), 1)
		o.Expect(err).To(o.Equal(wait.ErrWaitTimeout))
	})
})

func waitForRCToBeScaled(c kclient.ReplicationControllerInterface, rcName string, target int) error {
	return wait.PollImmediate(20*time.Second, 3*time.Minute, func() (bool, error) {
		if rc, err := c.Get(rcName); err != nil {
			return false, err
		} else if rc.Spec.Replicas == target {
			return true, nil
		}

		return false, nil
	})
}
