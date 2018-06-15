package trigger

import (
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var (
	SyncTimeout = 30 * time.Second
)

var _ = g.Describe("[Feature:AnnotationTrigger] Annotation trigger", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("cli-deployment", exutil.KubeConfigPath())

	var (
		deploymentFixture = exutil.FixturePath("testdata", "image", "deployment-with-annotation-trigger.yaml")
	)

	g.It("reconciles after the image is overwritten", func() {
		namespace := oc.Namespace()

		g.By("creating a Deployment")
		deployment, err := readDeploymentFixture(deploymentFixture)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(deployment.Spec.Template.Spec.Containers).To(o.HaveLen(1))
		o.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(o.Equal(" "))

		deployment, err = oc.KubeClient().AppsV1().Deployments(namespace).Create(deployment)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(o.Equal(" "))

		g.By("tagging the docker.io/library/centos:latest as test:v1 image to create ImageStream")
		out, err := oc.Run("tag").Args("docker.io/library/centos:latest", "test:v1").Output()
		framework.Logf("%s", out)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for the initial image to be replaced from ImageStream")
		deployment, err = waitForDeploymentModification(oc.KubeClient().AppsV1(), deployment.ObjectMeta, SyncTimeout, func(d *appsv1.Deployment) (bool, error) {
			return d.Spec.Template.Spec.Containers[0].Image != deployment.Spec.Template.Spec.Containers[0].Image, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("setting Deployment image repeatedly to ' ' to fight with annotation trigger")
		for i := 0; i < 50; i++ {
			deployment, err = oc.KubeClient().AppsV1().Deployments(namespace).Patch(deployment.Name, types.StrategicMergePatchType,
				[]byte(`{"spec":{"template":{"spec":{"containers":[{"name":"test","image":" "}]}}}}`))
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(o.Equal(" "))
		}

		g.By("waiting for the image to be injected by annotation trigger")
		o.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(o.Equal(" "))
		deployment, err = waitForDeploymentModification(oc.KubeClient().AppsV1(), deployment.ObjectMeta, SyncTimeout, func(d *appsv1.Deployment) (bool, error) {
			return d.Spec.Template.Spec.Containers[0].Image != deployment.Spec.Template.Spec.Containers[0].Image, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
