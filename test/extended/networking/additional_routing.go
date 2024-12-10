package networking

import (
	"context"
	"errors"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	openshiftapiv1 "github.com/openshift/api/operator/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	restclient "k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

const (
	frrk8sNamespace = "openshift-frr-k8s"
	statusSuccess   = "success"
)

var _ = g.Describe("[sig-network][Feature:AdditionalRoutingCapabilities][OCPFeatureGate:AdditionalRoutingCapabilities]", func() {
	oc := exutil.NewCLIWithPodSecurityLevel("advanced-routing", admissionapi.LevelBaseline)
	var cs *dynClientSet
	var crdClient *apiextensionsclientset.Clientset
	nodes := map[string]struct{}{}

	g.BeforeEach(func() {
		flipAdvancedRouting(oc, true)

		nodesList, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to list nodes")
		for _, n := range nodesList.Items {
			nodes[n.Name] = struct{}{}
		}
		cs, err = newDynClientSet()
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create dynamic client set")
		kubeConfig := restclient.CopyConfig(oc.AdminConfig())
		kubeConfig.QPS = 99999
		kubeConfig.Burst = 9999
		crdClient = apiextensionsclientset.NewForConfigOrDie(kubeConfig)
	})

	g.AfterEach(func() {
		g.By("deleting all the frr configurations")
		configs, err := cs.frrConfigurations().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to list frr configs")
		for _, c := range configs.Items {
			err := cs.deleteFRRConfiguration(c.GetName())
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete frr config", c.GetName())
		}

		o.Eventually(func() int {
			configs, err := cs.frrConfigurations().List(context.Background(), metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to list frr configs")
			return len(configs.Items)
		}, 3*time.Minute, 5*time.Second).Should(o.Equal(0))

		g.By("removing advanced routing capabilities")
		flipAdvancedRouting(oc, false)
	})

	g.It("should deploy frr-k8s pods when enabled", func() {

		g.By("checking that one frr-k8s pod exist for each node")
		o.Eventually(func() map[string]struct{} {
			pods, err := oc.AdminKubeClient().CoreV1().Pods(frrk8sNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: "app=frr-k8s"})
			if err != nil {
				return nil
			}
			return nodesForPods(pods.Items)
		}, 3*time.Minute, 5*time.Second).Should(o.Equal(nodes))

		g.By("checking the webhook endpoint is running")
		o.Eventually(func() error {
			pods, err := oc.AdminKubeClient().CoreV1().Pods(frrk8sNamespace).List(context.Background(), metav1.ListOptions{LabelSelector: "component=frr-k8s-webhook-server"})
			if err != nil {
				return err
			}
			if len(pods.Items) > 1 {
				return errors.New("found more than one webhook endpoint")
			}
			if len(pods.Items) == 0 {
				return errors.New("found no webhook endpoints")
			}
			return nil
		}, 3*time.Minute, 5*time.Second).ShouldNot(o.HaveOccurred())

		g.By("checking that all frr-k8s pods are running")
		o.Eventually(func() error {
			pods, err := oc.AdminKubeClient().CoreV1().Pods(frrk8sNamespace).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				return err
			}
			for _, pod := range pods.Items {
				if pod.Status.Phase != corev1.PodRunning {
					return fmt.Errorf("pod %s is not running", pod.Name)
				}
			}
			return nil
		}, 3*time.Minute, 5*time.Second).ShouldNot(o.HaveOccurred())
	})

	g.It("should deploy frr-k8s crds when enabled", func() {
		o.Eventually(func() error {
			_, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "frrconfigurations.frrk8s.metallb.io", metav1.GetOptions{})
			return err
		}, 3*time.Minute, 5*time.Second).ShouldNot(o.HaveOccurred())
	})

	g.It("should reflect the frr status when an frrconfiguration is applied", func() {
		err := cs.addFRRConfigurationWithASN("test", 123)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to add frr configuration")

		o.Eventually(func() error {
			for n := range nodes {
				status, err := cs.getFRRNodeStatus(n)
				if err != nil {
					return fmt.Errorf("failed to get status for %s: %w", n, err)
				}
				if status.lastConversion != statusSuccess {
					return fmt.Errorf("last conversion for %s is not success: %s", n, status.lastConversion)
				}
				if status.lastReload != statusSuccess {
					return fmt.Errorf("last reload for %s is not success: %s", n, status.lastConversion)
				}
			}
			return nil
		}, 3*time.Minute, 5*time.Second).ShouldNot(o.HaveOccurred())
	})

	g.It("should report invalid configurations", func() {
		err := cs.addFRRConfigurationWithASN("test", 123)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to add frr configuration")

		g.By("adding a conflicting router")

		err = cs.addFRRConfigurationWithASN("test1", 124)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to add frr configuration")

		o.Eventually(func() error {
			for n := range nodes {
				status, err := cs.getFRRNodeStatus(n)
				if err != nil {
					return fmt.Errorf("failed to get status for %s: %w", n, err)
				}
				if status.lastConversion == statusSuccess {
					return fmt.Errorf("last conversion for %s is not success", n)
				}
			}
			return nil
		}, 3*time.Minute, 5*time.Second).ShouldNot(o.HaveOccurred())
	})

	g.It("should remove the frr pods when disabled", func() {
		flipAdvancedRouting(oc, false)

		o.Eventually(func() int {
			pods, err := oc.AdminKubeClient().CoreV1().Pods(frrk8sNamespace).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				return 0
			}
			return len(pods.Items)
		}, 3*time.Minute, 5*time.Second).Should(o.Equal(0))
	})

	g.It("should not remove the crds when disabled", func() {
		flipAdvancedRouting(oc, false)
		o.Consistently(func() error {
			_, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "frrconfigurations.frrk8s.metallb.io", metav1.GetOptions{})
			return err
		}, 2*time.Minute, 5*time.Second).ShouldNot(o.HaveOccurred())
	})
})

func nodesForPods(pods []corev1.Pod) map[string]struct{} {
	res := map[string]struct{}{}
	for _, pod := range pods {
		res[pod.Spec.NodeName] = struct{}{}
	}
	return res
}

func flipAdvancedRouting(oc *exutil.CLI, enable bool) {
	o.Eventually(func() error {
		network, err := oc.AdminOperatorClient().OperatorV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("unable to get network config: %w", err)
		}
		network.Spec.AdditionalRoutingCapabilities = nil
		if enable {
			network.Spec.AdditionalRoutingCapabilities = &openshiftapiv1.AdditionalRoutingCapabilities{
				Providers: []openshiftapiv1.RoutingCapabilitiesProvider{
					openshiftapiv1.RoutingCapabilitiesProviderFRR,
				},
			}
		}
		_, err = oc.AdminOperatorClient().OperatorV1().Networks().Update(context.Background(), network, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("unable to update network config: %w", err)
		}
		return nil
	}, 3*time.Minute, 5*time.Second).ShouldNot(o.HaveOccurred())
}

type dynClientSet struct {
	dc dynamic.Interface
}

func (dcs dynClientSet) frrConfigurations() dynamic.ResourceInterface {
	return dcs.dc.Resource(schema.GroupVersionResource{Group: "frrk8s.metallb.io", Resource: "frrconfigurations", Version: "v1beta1"}).Namespace(frrk8sNamespace)
}

func (dcs dynClientSet) frrConfigurationStatuses() dynamic.ResourceInterface {
	return dcs.dc.Resource(schema.GroupVersionResource{Group: "frrk8s.metallb.io", Resource: "frrnodestates", Version: "v1beta1"})
}

func newDynClientSet() (*dynClientSet, error) {
	cfg, err := e2e.LoadConfig()
	if err != nil {
		return nil, err
	}

	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &dynClientSet{
		dc: dc,
	}, nil
}

func (client *dynClientSet) addFRRConfigurationWithASN(name string, asn int) error {
	frrConfiguration := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "frrk8s.metallb.io/v1beta1",
			"kind":       "FRRConfiguration",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": frrk8sNamespace,
			},
			"spec": map[string]interface{}{
				"bgp": map[string]interface{}{
					"routers": []map[string]interface{}{
						{
							"asn": asn,
						},
					},
				},
			},
		},
	}
	_, err := client.frrConfigurations().Create(context.TODO(), frrConfiguration, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

type frrNodeStatus struct {
	lastConversion string
	lastReload     string
}

func (client *dynClientSet) getFRRNodeStatus(name string) (frrNodeStatus, error) {
	res := frrNodeStatus{}

	result, getErr := client.frrConfigurationStatuses().Get(context.TODO(), name, metav1.GetOptions{})
	if getErr != nil {
		return res, fmt.Errorf("failed to get frr config status %s, %w", name, getErr)
	}

	lastConversionRes, found, err := unstructured.NestedString(result.Object, "status", "lastConversionResult")
	if err != nil {
		return res, fmt.Errorf("failed to parse lastConversionResult for %s: %w", name, err)
	}
	if !found {
		return res, fmt.Errorf("lastConversionResult field not found for %s", name)
	}
	lastReloadRes, found, err := unstructured.NestedString(result.Object, "status", "lastReloadResult")
	if err != nil {
		return res, fmt.Errorf("failed to parse lastReloadRes for %s: %w", name, err)
	}

	res.lastConversion = lastConversionRes
	res.lastReload = lastReloadRes

	return res, nil
}

func (client *dynClientSet) deleteFRRConfiguration(name string) error {
	err := client.frrConfigurations().Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}
