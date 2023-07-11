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

		g.By("wait until idling-echo endpoint is ready")
		err = wait.PollImmediate(time.Second, 60*time.Second, func() (done bool, err error) {
			err = oc.Run("describe").Args("endpoints", "idling-echo").Execute()
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("wait until replicationcontroller is ready")
		err = wait.PollImmediate(time.Second, 60*time.Second, func() (done bool, err error) {
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
		err = wait.PollImmediate(time.Second, 60*time.Second, func() (done bool, err error) {
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
		err = wait.PollImmediate(time.Second, 60*time.Second, func() (done bool, err error) {
			out, err := oc.Run("get").Args("endpoints", "idling-echo", "--template={{ len (index .subsets 0).addresses }}", "--output=go-template").Output()
			if err != nil || out != scaledReplicaCount {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("by name", func() {
		err := oc.Run("idle").Args(fmt.Sprintf("dc/%s", deploymentConfigName)).Execute()
		o.Expect(err).To(o.HaveOccurred())

		out, err := oc.Run("idle").Args("idling-echo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(expectedOutput))

		out, err = oc.Run("get").Args("endpoints", "idling-echo", idledTemplate, "--output=go-template").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.BeEmpty())
	})

	g.It("by label", func() {
		out, err := oc.Run("idle").Args("-l", "app=idling-echo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(expectedOutput))

		out, err = oc.Run("get").Args("endpoints", "idling-echo", idledTemplate, "--output=go-template").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.BeEmpty())
	})

	g.It("by all", func() {
		out, err := oc.Run("idle").Args("--all").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(expectedOutput))

		out, err = oc.Run("get").Args("endpoints", "idling-echo", idledTemplate, "--output=go-template").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.BeEmpty())
	})

	g.It("by checking previous scale", func() {
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
