package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	idledAnnotation     = "idling.alpha.openshift.io/idled-at"
	prevScaleAnnotation = "idling.alpha.openshift.io/previous-scale"
	scaledReplicaCount  = "2"
)

var _ = g.Describe("[sig-cli] oc idle [apigroup:apps.openshift.io][apigroup:route.openshift.io][apigroup:project.openshift.io][apigroup:image.openshift.io]", func() {
	defer g.GinkgoRecover()

	var (
		oc                   = exutil.NewCLIWithPodSecurityLevel("oc-idle", admissionapi.LevelBaseline)
		cmdTestData          = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata")
		idleSVCRoute         = filepath.Join(cmdTestData, "idling-svc-route.yaml")
		idleDeploymentConfig = filepath.Join(cmdTestData, "idling-dc.yaml")
		idledTemplate        = fmt.Sprintf("--template={{index .metadata.annotations \"%s\"}}", idledAnnotation)
	)

	var deploymentConfigName, expectedOutput string
	g.JustBeforeEach(func() {
		projectName, err := oc.Run("project").Args("-q").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create required service and routers")
		err = oc.Run("create").Args("-f", idleSVCRoute).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create deploymentconfig and get deploymentconfig name")
		_, err = oc.Run("create").Args("-f", idleDeploymentConfig).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		dcList, err := oc.AdminAppsClient().AppsV1().DeploymentConfigs(projectName).List(context.TODO(), metav1.ListOptions{LabelSelector: "app=idling-echo,deploymentconfig=idling-echo"})
		o.Expect(dcList.Items).Should(o.HaveLen(1))
		deploymentConfigName = dcList.Items[0].Name

		expectedOutput = fmt.Sprintf("The service will unidle DeploymentConfig \"%s/%s\" to %s replicas once it receives traffic", projectName, deploymentConfigName, scaledReplicaCount)

		err = oc.Run("describe").Args("deploymentconfigs", deploymentConfigName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		ctx := context.Background()
		g.By("wait until idling-echo endpoint is ready")
		err = wait.PollUntilContextTimeout(ctx, time.Second, 60*time.Second, true, func(ctx context.Context) (done bool, err error) {
			err = oc.Run("describe").Args("endpointslices", "-l", "kubernetes.io/service-name=idling-echo").Execute()
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("wait until replicationcontroller is ready")
		err = wait.PollUntilContextTimeout(ctx, time.Second, 60*time.Second, true, func(ctx context.Context) (done bool, err error) {
			err = oc.Run("get").Args("replicationcontroller", fmt.Sprintf("%s-1", deploymentConfigName)).Execute()
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("scale deploymentconfig to %s replicas", scaledReplicaCount))
		err = oc.Run("scale").Args("replicationcontroller", fmt.Sprintf("%s-1", deploymentConfigName), fmt.Sprintf("--replicas=%s", scaledReplicaCount)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("wait until pod is scaled to %s", scaledReplicaCount))
		err = wait.PollUntilContextTimeout(ctx, time.Second, 60*time.Second, true, func(ctx context.Context) (done bool, err error) {
			out, err := oc.Run("get").Args("pods", "-l", "app=idling-echo", "--template={{ len .items }}", "--output=go-template").Output()
			if err != nil {
				return false, err
			}

			if out != scaledReplicaCount {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("wait until endpoint addresses are scaled to %s", scaledReplicaCount))
		err = wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true, func(ctx context.Context) (done bool, err error) {
			out, err := oc.Run("get").Args("endpointslices", "-l", "kubernetes.io/service-name=idling-echo", "--template={{ len (index .items 0).endpoints }}", "--output=go-template").Output()
			if err != nil || out != scaledReplicaCount {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("by name", g.Label("Size:M"), func() {
		err := oc.Run("idle").Args(fmt.Sprintf("dc/%s", deploymentConfigName)).Execute()
		o.Expect(err).To(o.HaveOccurred())

		out, err := oc.Run("idle").Args("idling-echo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(expectedOutput))

		out, err = oc.Run("get").Args("service", "idling-echo", idledTemplate, "--output=go-template").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.BeEmpty())
	})

	g.It("by label", g.Label("Size:M"), func() {
		out, err := oc.Run("idle").Args("-l", "app=idling-echo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(expectedOutput))

		out, err = oc.Run("get").Args("service", "idling-echo", idledTemplate, "--output=go-template").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.BeEmpty())
	})

	g.It("by all", g.Label("Size:M"), func() {
		out, err := oc.Run("idle").Args("--all").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(expectedOutput))

		out, err = oc.Run("get").Args("service", "idling-echo", idledTemplate, "--output=go-template").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.BeEmpty())
	})

	g.It("by checking previous scale", g.Label("Size:M"), func() {
		out, err := oc.Run("idle").Args("idling-echo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(expectedOutput))

		projectName, err := oc.Run("project").Args("-q").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		dcObj, err := oc.AdminAppsClient().AppsV1().DeploymentConfigs(projectName).Get(context.TODO(), deploymentConfigName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		out = dcObj.Annotations[prevScaleAnnotation]
		o.Expect(out).To(o.Equal(scaledReplicaCount))
	})
})

var _ = g.Describe("[sig-cli] oc idle Deployments [apigroup:route.openshift.io][apigroup:project.openshift.io][apigroup:image.openshift.io]", func() {
	defer g.GinkgoRecover()

	var (
		oc             = exutil.NewCLIWithPodSecurityLevel("oc-idle", admissionapi.LevelBaseline)
		cmdTestData    = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata")
		idleSVCRoute   = filepath.Join(cmdTestData, "idling-svc-route.yaml")
		idleDeployment = filepath.Join(cmdTestData, "idling-deployment.yaml")
		idledTemplate  = fmt.Sprintf("--template={{index .metadata.annotations \"%s\"}}", idledAnnotation)
	)

	var deploymentName, expectedOutput string
	g.JustBeforeEach(func() {
		projectName, err := oc.Run("project").Args("-q").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create required service and routers")
		err = oc.Run("create").Args("-f", idleSVCRoute).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create deployment and get deployment name")
		_, err = oc.Run("create").Args("-f", idleDeployment).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		dcList, err := oc.AdminKubeClient().AppsV1().Deployments(projectName).List(context.TODO(), metav1.ListOptions{LabelSelector: "app=idling-echo,deployment=idling-echo"})
		o.Expect(dcList.Items).Should(o.HaveLen(1))
		deploymentName = dcList.Items[0].Name

		expectedOutput = fmt.Sprintf("The service will unidle Deployment \"%s/%s\" to %s replicas once it receives traffic", projectName, deploymentName, scaledReplicaCount)

		err = oc.Run("describe").Args("deployments", deploymentName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		ctx := context.Background()
		g.By("wait until idling-echo endpoint is ready")
		err = wait.PollUntilContextTimeout(ctx, time.Second, 60*time.Second, true, func(ctx context.Context) (done bool, err error) {
			err = oc.Run("describe").Args("endpointslices", "-l", "kubernetes.io/service-name=idling-echo").Execute()
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("wait until replicaset is ready")
		var rsName string
		err = wait.PollUntilContextTimeout(ctx, time.Second, 60*time.Second, true, func(ctx context.Context) (done bool, err error) {
			rsList, err := oc.AdminKubeClient().AppsV1().ReplicaSets(projectName).List(context.TODO(), metav1.ListOptions{LabelSelector: "app=idling-echo,deployment=idling-echo"})
			o.Expect(err).NotTo(o.HaveOccurred())
			if len(rsList.Items) != 1 {
				klog.Infof("Expected only a single replicaset, got %d instead", len(rsList.Items))
				return false, nil
			}
			rsName = rsList.Items[0].Name
			err = oc.Run("get").Args("replicaset", rsName).Execute()
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("scale deployment to %s replicas", scaledReplicaCount))
		err = oc.Run("scale").Args("replicaset", rsName, fmt.Sprintf("--replicas=%s", scaledReplicaCount)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("wait until pod is scaled to %s", scaledReplicaCount))
		err = wait.PollUntilContextTimeout(ctx, time.Second, 60*time.Second, true, func(ctx context.Context) (done bool, err error) {
			out, err := oc.Run("get").Args("pods", "-l", "app=idling-echo", "--template={{ len .items }}", "--output=go-template").Output()
			if err != nil {
				return false, err
			}

			if out != scaledReplicaCount {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("wait until endpoint addresses are scaled to %s", scaledReplicaCount))
		err = wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true, func(ctx context.Context) (done bool, err error) {
			out, err := oc.Run("get").Args("endpointslices", "-l", "kubernetes.io/service-name=idling-echo", "--template={{ len (index .items 0).endpoints }}", "--output=go-template").Output()
			if err != nil || out != scaledReplicaCount {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("by name", g.Label("Size:M"), func() {
		err := oc.Run("idle").Args(fmt.Sprintf("deployment/%s", deploymentName)).Execute()
		o.Expect(err).To(o.HaveOccurred())

		out, err := oc.Run("idle").Args("idling-echo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(expectedOutput))

		out, err = oc.Run("get").Args("service", "idling-echo", idledTemplate, "--output=go-template").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.BeEmpty())
	})

	g.It("by label", g.Label("Size:M"), func() {
		out, err := oc.Run("idle").Args("-l", "app=idling-echo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(expectedOutput))

		out, err = oc.Run("get").Args("service", "idling-echo", idledTemplate, "--output=go-template").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.BeEmpty())
	})

	g.It("by all", g.Label("Size:M"), func() {
		out, err := oc.Run("idle").Args("--all").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(expectedOutput))

		out, err = oc.Run("get").Args("service", "idling-echo", idledTemplate, "--output=go-template").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.BeEmpty())
	})
})
