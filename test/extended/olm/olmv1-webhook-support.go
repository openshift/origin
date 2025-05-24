package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"path/filepath"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	conditionTypeServing   = "Serving"
	conditionTypeInstalled = "Installed"
)

var (
	webhookTestBaseDir = exutil.FixturePath("testdata", "olmv1", "webhook-support")
)

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLMWebhookProviderOpenshiftServiceCA][Skipped:Disconnected] OLMv1 support for bundles with webhooks", g.Ordered, func() {
	defer g.GinkgoRecover()
	var (
		oc                 = exutil.NewCLI("openshift-operator-controller-webhooks")
		v1WbhookTestClient = getDynamicClient(getRestConfig(exutil.KubeConfigPath())).Resource(schema.GroupVersionResource{
			Group:    "webhook.operators.coreos.io",
			Version:  "v1",
			Resource: "webhooktests",
		})
		v2WbhookTestClient = getDynamicClient(getRestConfig(exutil.KubeConfigPath())).Resource(schema.GroupVersionResource{
			Group:    "webhook.operators.coreos.io",
			Version:  "v2",
			Resource: "webhooktests",
		})
	)

	g.BeforeAll(func(ctx context.Context) {
		g.By("checking olmv1 capability is enabled")
		checkFeatureCapability(oc)

		g.By("creating the webhook-operator catalog")
		cleanupCatalog := installWebhookOperatorClusterCatalog(ctx, oc)
		g.DeferCleanup(cleanupCatalog)

		g.By("installing the webhook operator")
		cleanupExtension := installWebhookOperator(ctx, oc)
		g.DeferCleanup(cleanupExtension)
	})

	g.BeforeEach(func() {
		exutil.PreTestDump()
	})

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			exutil.DumpPodLogsStartingWith("", oc)
		}
	})

	g.It("should have working validating webhook", func(ctx g.SpecContext) {
		g.By("creating a webhook test resource that will be rejected by the validating webhook")
		file := filepath.Join(webhookTestBaseDir, "webhook-test-validating-reject.yaml")
		_, err := apply(oc, file)
		o.Expect(err).To(o.HaveOccurred())

		g.By("validating the error message")
		o.Expect(err.Error()).To(o.ContainSubstring("Invalid value: false: Spec.Valid must be true"))
	})

	g.It("should have working mutating webhook", func(ctx g.SpecContext) {
		g.By("creating a valid webhook test resource")
		file := filepath.Join(webhookTestBaseDir, "webhook-test-accept.yaml")
		cleanup, err := apply(oc, file)
		g.DeferCleanup(cleanup)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("getting the created resource in v1 schema")
		obj, err := v1WbhookTestClient.Namespace("webhook-operator").Get(ctx, "webhook-test", metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(obj).ToNot(o.BeNil())

		g.By("validating the resource spec")
		spec := obj.Object["spec"].(map[string]interface{})
		o.Expect(spec).To(o.Equal(map[string]interface{}{
			"valid":  true,
			"mutate": true,
		}))
	})

	g.It("should have working conversion webhook", func(ctx g.SpecContext) {
		file := filepath.Join(webhookTestBaseDir, "webhook-test-accept.yaml")
		g.By("creating a valid webhook test resource")
		cleanup, err := apply(oc, file)
		g.DeferCleanup(cleanup)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("getting the created resource in v2 schema")
		obj, err := v2WbhookTestClient.Namespace("webhook-operator").Get(ctx, "webhook-test", metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(obj).ToNot(o.BeNil())

		g.By("validating the resource spec")
		spec := obj.Object["spec"].(map[string]interface{})
		o.Expect(spec).To(o.Equal(map[string]interface{}{
			"conversion": map[string]interface{}{
				"valid":  true,
				"mutate": true,
			},
		}))
	})
	//
	//g.It("should have cert secret", func(ctx g.SpecContext) {
	//
	//})
	//
	//g.It("should do one last thing for some reason", func(ctx g.SpecContext) {
	//
	//})
})

func installWebhookOperatorClusterCatalog(ctx context.Context, oc *exutil.CLI) func() {
	webhookCatalogYaml := filepath.Join(webhookTestBaseDir, "webhook-operator-catalog.yaml")
	cleanup, err := applyResourceFileByPath(oc, webhookCatalogYaml)
	g.DeferCleanup(cleanup)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("waiting for the webhook-operator ClusterCatalog to be serving")
	var lastReason string
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			isDone, err, status := checkWebhookCatalogServing(oc)
			if lastReason != status {
				g.GinkgoLogr.Info(fmt.Sprintf("waitForWebhookOperatorCatalogServing: %q", status))
				lastReason = status
			}
			return isDone, err
		})
	o.Expect(lastReason).To(o.BeEmpty())
	o.Expect(err).NotTo(o.HaveOccurred())
	return cleanup
}

func installWebhookOperator(ctx context.Context, oc *exutil.CLI) func() {
	file := filepath.Join(webhookTestBaseDir, "webhook-operator.yaml")
	cleanup, err := applyResourceFileByPath(oc, file)
	o.Expect(err).NotTo(o.HaveOccurred())
	g.DeferCleanup(cleanup)

	g.By("waiting for the webhook-operator bundle to be installed")
	var lastReason string
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			isDone, err, status := checkWebhookOperatorInstalled(oc)
			if lastReason != status {
				g.GinkgoLogr.Info(fmt.Sprintf("waitForWebhookOperatorInstalled: %q", status))
				lastReason = status
			}
			return isDone, err
		})
	o.Expect(lastReason).To(o.BeEmpty())
	o.Expect(err).NotTo(o.HaveOccurred())
	return cleanup
}

func applyResourceFileByPath(oc *exutil.CLI, filePath string) (func(), error) {
	g.By(fmt.Sprintf("applying the necessary %q resources", filePath))
	err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", filePath).Execute()
	return func() {
		g.By(fmt.Sprintf("cleaning the necessary %q resources", filePath))
		err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", filePath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}, err
}

func checkWebhookCatalogServing(oc *exutil.CLI) (bool, error, string) {
	cmdArgs := []string{
		"clustercatalogs.olm.operatorframework.io",
		"webhook-operator-catalog",
		"-o=jsonpath={.status.conditions}",
	}

	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(cmdArgs...).Output()
	if err != nil {
		return false, err, ""
	}
	// no data yet, so try again
	if output == "" {
		return false, nil, "no output"
	}

	var conditions []metav1.Condition
	if err := json.Unmarshal([]byte(output), &conditions); err != nil {
		return false, fmt.Errorf("error in json.Unmarshal(%v): %v", output, err), ""
	}
	c := meta.FindStatusCondition(conditions, conditionTypeServing)
	if c == nil {
		return false, nil, fmt.Sprintf("condition not present: %q", conditionTypeServing)
	}
	if c.Status != metav1.ConditionTrue {
		return false, nil, fmt.Sprintf("expected status to be %q: %+v", metav1.ConditionTrue, c)
	}
	return true, nil, ""
}

func checkWebhookOperatorInstalled(oc *exutil.CLI) (bool, error, string) {
	cmdArgs := []string{
		"clusterextensions.olm.operatorframework.io",
		"webhook-operator",
		"-o=jsonpath={.status.conditions}",
	}

	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(cmdArgs...).Output()
	if err != nil {
		return false, err, ""
	}
	// no data yet, so try again
	if output == "" {
		return false, nil, "no output"
	}

	var conditions []metav1.Condition
	if err := json.Unmarshal([]byte(output), &conditions); err != nil {
		return false, fmt.Errorf("error in json.Unmarshal(%v): %v", output, err), ""
	}
	c := meta.FindStatusCondition(conditions, conditionTypeInstalled)
	if c == nil {
		return false, nil, fmt.Sprintf("condition not present: %q", conditionTypeInstalled)
	}
	if c.Status != metav1.ConditionTrue {
		return false, nil, fmt.Sprintf("expected status to be %q: %+v", metav1.ConditionTrue, c)
	}
	return true, nil, ""
}

func apply(oc *exutil.CLI, filePath string, cmdOpts ...string) (func(), error) {
	g.By(fmt.Sprintf("applying the necessary %q resources", filePath))
	err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", filePath).Execute()
	return func() {
		g.By(fmt.Sprintf("cleaning the necessary %q resources", filePath))
		err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", filePath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}, err
}

func getRestConfig(kubeConfigPath string) *rest.Config {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	o.Expect(err).NotTo(o.HaveOccurred())
	return config
}

func getDynamicClient(restConfig *rest.Config) dynamic.Interface {
	client, err := dynamic.NewForConfig(restConfig)
	o.Expect(err).NotTo(o.HaveOccurred())
	return client
}
