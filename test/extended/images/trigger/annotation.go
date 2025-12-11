package trigger

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	exutilimage "github.com/openshift/origin/test/extended/util/image"
)

var (
	SyncTimeout = 30 * time.Second
)

var _ = g.Describe("[sig-imageregistry][Feature:ImageTriggers] Annotation trigger", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("cli-deployment")

	var (
		deploymentFixture = exutil.FixturePath("testdata", "image", "deployment-with-annotation-trigger.yaml")
	)

	g.It("reconciles after the image is overwritten [apigroup:image.openshift.io]", g.Label("Size:M"), func() {
		namespace := oc.Namespace()

		g.By("creating a Deployment")
		deployment, err := readDeploymentFixture(deploymentFixture)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(deployment.Spec.Template.Spec.Containers).To(o.HaveLen(1))
		o.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(o.Equal(" "))

		deployment, err = oc.KubeClient().AppsV1().Deployments(namespace).Create(context.Background(), deployment, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(o.Equal(" "))

		g.By("tagging a new image as test:v1 image to create ImageStream")
		out, err := oc.Run("tag").Args(exutilimage.ShellImage(), "test:v1").Output()
		framework.Logf("%s", out)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for the initial image to be replaced from ImageStream")
		deployment, err = waitForDeploymentModification(oc.KubeClient().AppsV1(), deployment.ObjectMeta, SyncTimeout, func(d *appsv1.Deployment) (bool, error) {
			return d.Spec.Template.Spec.Containers[0].Image != deployment.Spec.Template.Spec.Containers[0].Image, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("setting Deployment image repeatedly to ' ' to fight with annotation trigger")
		for i := 0; i < 50; i++ {
			deployment, err = oc.KubeClient().AppsV1().Deployments(namespace).Patch(context.Background(), deployment.Name, types.StrategicMergePatchType,
				[]byte(`{"spec":{"template":{"spec":{"containers":[{"name":"test","image":" "}]}}}}`), metav1.PatchOptions{})
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
