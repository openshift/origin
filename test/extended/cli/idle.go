package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	idledAnnotation     = "idling.alpha.openshift.io/idled-at"
	prevScaleAnnotation = "idling.alpha.openshift.io/previous-scale"
	scaledReplicaCount  = 2
)

func readyPodEndpointCount(endpointSlices []discoveryv1.EndpointSlice) int {
	readyPods := map[string]struct{}{}
	for _, endpointSlice := range endpointSlices {
		for _, endpoint := range endpointSlice.Endpoints {
			if endpoint.TargetRef == nil || endpoint.TargetRef.Kind != "Pod" {
				continue
			}
			if endpoint.Conditions.Ready == nil || *endpoint.Conditions.Ready {
				key := fmt.Sprintf("%s/%s/%s", endpoint.TargetRef.Namespace, endpoint.TargetRef.Name, endpoint.TargetRef.UID)
				readyPods[key] = struct{}{}
			}
		}
	}
	return len(readyPods)
}

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

		expectedOutput = fmt.Sprintf("The service will unidle DeploymentConfig \"%s/%s\" to %d replicas once it receives traffic", projectName, deploymentConfigName, scaledReplicaCount)

		err = oc.Run("describe").Args("deploymentconfigs", deploymentConfigName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		ctx := context.Background()
		g.By("wait until replicationcontroller exists")
		err = wait.PollUntilContextTimeout(ctx, time.Second, 60*time.Second, true, func(ctx context.Context) (done bool, err error) {
			err = oc.Run("get").Args("replicationcontroller", fmt.Sprintf("%s-1", deploymentConfigName)).Execute()
			if err != nil {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("scale deploymentconfig to %d replicas", scaledReplicaCount))
		err = oc.Run("scale").Args("replicationcontroller", fmt.Sprintf("%s-1", deploymentConfigName), fmt.Sprintf("--replicas=%d", scaledReplicaCount)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("wait until pod is scaled to %d", scaledReplicaCount))
		err = wait.PollUntilContextTimeout(ctx, time.Second, 60*time.Second, true, func(ctx context.Context) (done bool, err error) {
			out, err := oc.Run("get").Args("pods", "-l", "app=idling-echo", "--template={{ len .items }}", "--output=go-template").Output()
			if err != nil {
				return false, err
			}

			if out != fmt.Sprint(scaledReplicaCount) {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("wait until %d ready pod-backed endpoints are available", scaledReplicaCount))
		err = wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true, func(ctx context.Context) (done bool, err error) {
			endpointSlices, err := oc.KubeClient().DiscoveryV1().EndpointSlices(projectName).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", discoveryv1.LabelServiceName, "idling-echo")})
			if err != nil {
				return false, nil
			}

			rc, err := oc.KubeClient().CoreV1().ReplicationControllers(projectName).Get(ctx, fmt.Sprintf("%s-1", deploymentConfigName), metav1.GetOptions{})
			if err != nil {
				return false, nil
			}

			return rc.Status.ReadyReplicas == int32(scaledReplicaCount) && readyPodEndpointCount(endpointSlices.Items) == scaledReplicaCount, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("by name", func() {
		err := oc.Run("idle").Args(fmt.Sprintf("dc/%s", deploymentConfigName)).Execute()
		o.Expect(err).To(o.HaveOccurred())

		out, err := oc.Run("idle").Args("idling-echo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(expectedOutput))

		out, err = oc.Run("get").Args("service", "idling-echo", idledTemplate, "--output=go-template").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.BeEmpty())
	})

	g.It("by label", func() {
		out, err := oc.Run("idle").Args("-l", "app=idling-echo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(expectedOutput))

		out, err = oc.Run("get").Args("service", "idling-echo", idledTemplate, "--output=go-template").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.BeEmpty())
	})

	g.It("by all", func() {
		out, err := oc.Run("idle").Args("--all").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(expectedOutput))

		out, err = oc.Run("get").Args("service", "idling-echo", idledTemplate, "--output=go-template").Output()
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
		o.Expect(out).To(o.Equal(fmt.Sprint(scaledReplicaCount)))
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
		framework      = oc.KubeFramework()
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

		expectedOutput = fmt.Sprintf("The service will unidle Deployment \"%s/%s\" to %d replicas once it receives traffic", projectName, deploymentName, scaledReplicaCount)

		err = oc.Run("describe").Args("deployments", deploymentName).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		ctx := context.Background()
		g.By("wait until replicaset exists")
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

		g.By(fmt.Sprintf("scale deployment to %d replicas", scaledReplicaCount))
		err = oc.Run("scale").Args("replicaset", rsName, fmt.Sprintf("--replicas=%d", scaledReplicaCount)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("wait until pod is scaled to %d", scaledReplicaCount))
		err = wait.PollUntilContextTimeout(ctx, time.Second, 60*time.Second, true, func(ctx context.Context) (done bool, err error) {
			out, err := oc.Run("get").Args("pods", "-l", "app=idling-echo", "--template={{ len .items }}", "--output=go-template").Output()
			if err != nil {
				return false, err
			}

			if out != fmt.Sprint(scaledReplicaCount) {
				return false, nil
			}

			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("wait until %d ready pod-backed endpoints are available", scaledReplicaCount))
		err = wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true, func(ctx context.Context) (done bool, err error) {
			endpointSlices, err := oc.KubeClient().DiscoveryV1().EndpointSlices(projectName).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", discoveryv1.LabelServiceName, "idling-echo")})
			if err != nil {
				return false, nil
			}

			rs, err := oc.KubeClient().AppsV1().ReplicaSets(projectName).Get(ctx, rsName, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}

			return rs.Status.ReadyReplicas == int32(scaledReplicaCount) && readyPodEndpointCount(endpointSlices.Items) == scaledReplicaCount, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("by name", func() {
		err := oc.Run("idle").Args(fmt.Sprintf("deployment/%s", deploymentName)).Execute()
		o.Expect(err).To(o.HaveOccurred())

		out, err := oc.Run("idle").Args("idling-echo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(expectedOutput))

		out, err = oc.Run("get").Args("service", "idling-echo", idledTemplate, "--output=go-template").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.BeEmpty())

		g.By("wait until the deployment is idled")
		err = wait.PollUntilContextTimeout(context.Background(), time.Second, 5*time.Minute, true, func(ctx context.Context) (done bool, err error) {
			deployment, err := oc.KubeClient().AppsV1().Deployments(oc.Namespace()).Get(ctx, deploymentName, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			endpointSlices, err := oc.KubeClient().DiscoveryV1().EndpointSlices(oc.Namespace()).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", discoveryv1.LabelServiceName, "idling-echo")})
			if err != nil {
				return false, nil
			}
			return deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == 0 && deployment.Status.ReadyReplicas == 0 && readyPodEndpointCount(endpointSlices.Items) == 0, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("send traffic to unidle the deployment")
		service, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(context.Background(), "idling-echo", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		tcpPort := int32(0)
		for _, port := range service.Spec.Ports {
			if port.Protocol == "TCP" {
				tcpPort = port.Port
				break
			}
		}
		o.Expect(service.Spec.ClusterIP).NotTo(o.BeEmpty())
		o.Expect(tcpPort).NotTo(o.BeZero())

		execPod := e2epod.CreateExecPodOrFail(context.Background(), framework.ClientSet, framework.Namespace.Name, "execpod", nil)
		out, err = e2eoutput.RunHostCmd(execPod.Namespace, execPod.Name, fmt.Sprintf("echo -n wake | nc -N -w 120 %s %d", service.Spec.ClusterIP, tcpPort))
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Equal("wake"))

		g.By("wait until the deployment and its endpoints are ready after unidling")
		err = wait.PollUntilContextTimeout(context.Background(), time.Second, 5*time.Minute, true, func(ctx context.Context) (done bool, err error) {
			deployment, err := oc.KubeClient().AppsV1().Deployments(oc.Namespace()).Get(ctx, deploymentName, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			endpointSlices, err := oc.KubeClient().DiscoveryV1().EndpointSlices(oc.Namespace()).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", discoveryv1.LabelServiceName, "idling-echo")})
			if err != nil {
				return false, nil
			}
			return deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == int32(scaledReplicaCount) && deployment.Status.ReadyReplicas == int32(scaledReplicaCount) && readyPodEndpointCount(endpointSlices.Items) == scaledReplicaCount, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("by label", func() {
		out, err := oc.Run("idle").Args("-l", "app=idling-echo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(expectedOutput))

		out, err = oc.Run("get").Args("service", "idling-echo", idledTemplate, "--output=go-template").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.BeEmpty())
	})

	g.It("by all", func() {
		out, err := oc.Run("idle").Args("--all").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring(expectedOutput))

		out, err = oc.Run("get").Args("service", "idling-echo", idledTemplate, "--output=go-template").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.BeEmpty())
	})
})
