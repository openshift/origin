package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"os"
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

	openshiftServiceCANamespace            = "openshift-service-ca"
	openshiftServiceCASigningKeySecretName = "signing-key"
)

var (
	webhookTestBaseDir = exutil.FixturePath("testdata", "olmv1", "webhook-support")
)

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLMWebhookProviderOpenshiftServiceCA][Skipped:Disconnected][Serial] OLMv1 operator with webhooks", g.Ordered, g.Serial, func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("openshift-operator-controller-webhooks")
	)

	g.BeforeAll(func(ctx g.SpecContext) {
		webhookOperatorCatalogPath := filepath.Join(webhookTestBaseDir, "webhook-operator-catalog.yaml")
		webhookOperatorClusterExtPath := filepath.Join(webhookTestBaseDir, "webhook-operator.yaml")

		g.By("checking olmv1 capability is enabled")
		checkFeatureCapability(oc)

		g.By("creating the webhook-operator catalog")
		cleanupCatalog := installWebhookOperatorCatalog(ctx, webhookOperatorCatalogPath, oc)
		g.DeferCleanup(cleanupCatalog)

		g.By("installing the webhook operator")
		cleanupExtension := installWebhookOperatorExtension(ctx, webhookOperatorClusterExtPath, oc)
		g.DeferCleanup(cleanupExtension)

		resource, err := toUnstructured(webhookOperatorClusterExtPath)
		o.Expect(err).ToNot(o.HaveOccurred())

		g.By("waiting for the webhook operator deployment to be Available")
		err = oc.AsAdmin().WithoutNamespace().Run("wait").Args("--for=condition=Available", "-n", resource.GetNamespace(), "deployments/webhook-operator-webhook").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.BeforeEach(func() {
		exutil.PreTestDump()
	})

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			exutil.DumpPodLogsStartingWith("", oc)
		}
	})

	g.It("should have a working validating webhook", func(ctx g.SpecContext) {
		g.By("creating a webhook test resource that will be rejected by the validating webhook")
		file := filepath.Join(webhookTestBaseDir, "validating-webhook-test.yaml")
		// even if the resource gets created, it will be cleaned up when test gets torn down and the
		// webhook-operator namespace gets deleted
		_, err := apply(oc, file)
		o.Expect(err).To(o.HaveOccurred())

		g.By("validating the error message")
		o.Expect(err.Error()).To(o.ContainSubstring("Invalid value: false: Spec.Valid must be true"))
	})

	g.It("should have a working mutating webhook", func(ctx g.SpecContext) {
		g.By("creating a valid webhook test resource")
		file := filepath.Join(webhookTestBaseDir, "mutating-webhook-test.yaml")
		cleanup, err := apply(oc, file)
		g.DeferCleanup(cleanup)
		o.Expect(err).ToNot(o.HaveOccurred())

		resource, err := toUnstructured(file)
		o.Expect(err).ToNot(o.HaveOccurred())

		cfg, err := clientcmd.BuildConfigFromFlags("", exutil.KubeConfigPath())
		o.Expect(err).ToNot(o.HaveOccurred())
		c, err := dynamic.NewForConfig(cfg)
		o.Expect(err).ToNot(o.HaveOccurred())

		v1WebhookTestClient := c.Resource(schema.GroupVersionResource{
			Group:    "webhook.operators.coreos.io",
			Version:  "v1",
			Resource: "webhooktests",
		})

		g.By("getting the created resource in v1 schema")
		obj, err := v1WebhookTestClient.Namespace(resource.GetNamespace()).Get(ctx, resource.GetName(), metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())
		o.Expect(obj).ToNot(o.BeNil())

		g.By("validating the resource spec")
		spec := obj.Object["spec"].(map[string]interface{})
		o.Expect(spec).To(o.Equal(map[string]interface{}{
			"valid":  true,
			"mutate": true,
		}))
	})

	g.It("should have a working conversion webhook", func(ctx g.SpecContext) {
		file := filepath.Join(webhookTestBaseDir, "conversion-webhook-test.yaml")
		g.By("creating a valid webhook test resource")
		cleanup, err := apply(oc, file)
		g.DeferCleanup(cleanup)
		o.Expect(err).ToNot(o.HaveOccurred())

		resource, err := toUnstructured(file)
		o.Expect(err).ToNot(o.HaveOccurred())

		cfg, err := clientcmd.BuildConfigFromFlags("", exutil.KubeConfigPath())
		o.Expect(err).ToNot(o.HaveOccurred())
		c, err := dynamic.NewForConfig(cfg)
		o.Expect(err).ToNot(o.HaveOccurred())

		v2WebhookTestClient := c.Resource(schema.GroupVersionResource{
			Group:    "webhook.operators.coreos.io",
			Version:  "v2",
			Resource: "webhooktests",
		})

		g.By("getting the created resource in v2 schema")
		obj, err := v2WebhookTestClient.Namespace(resource.GetNamespace()).Get(ctx, resource.GetName(), metav1.GetOptions{})
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

	g.It("should be tolerant to openshift-service-ca certificate rotation", func(ctx g.SpecContext) {
		file := filepath.Join(webhookTestBaseDir, "mutating-webhook-test.yaml")

		// delete tls secret
		g.By("deleting the openshift-service-ca signing-key secret")
		err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("secret", openshiftServiceCASigningKeySecretName, "--namespace", openshiftServiceCANamespace).Execute()
		o.Expect(err).ToNot(o.HaveOccurred())

		// even with the secret deleted the webhook should still be responsive
		g.By("checking webhook is responsive through cert rotation")
		o.Eventually(func(g o.Gomega) {
			cleanup, err := apply(oc, file)
			g.Expect(err).ToNot(o.HaveOccurred())
			cleanup()
		}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})

	g.It("should still work if tls secret is deleted", func(ctx g.SpecContext) {
		file := filepath.Join(webhookTestBaseDir, "mutating-webhook-test.yaml")

		resource, err := toUnstructured(file)
		o.Expect(err).ToNot(o.HaveOccurred())

		certificateSecretName := "webhook-operator-webhook-service-cert"

		g.By("ensuring secret exists")
		o.Eventually(func(g o.Gomega) {
			err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", certificateSecretName, "-n", resource.GetNamespace()).Execute()
			o.Expect(err).ToNot(o.HaveOccurred())
		}).Should(o.Succeed())

		// delete tls secret
		g.By("checking webhook is responsive through secret recreation")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("secret", certificateSecretName, "-n", resource.GetNamespace()).Execute()
		o.Expect(err).ToNot(o.HaveOccurred())

		// even with the secret deleted the webhook should still be responsive
		o.Eventually(func(g o.Gomega) {
			cleanup, err := apply(oc, file)
			g.Expect(err).ToNot(o.HaveOccurred())
			cleanup()
		}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})
})

func toUnstructured(yamlPath string) (*unstructured.Unstructured, error) {
	f, err := os.Open(yamlPath)
	if err != nil {
		return nil, err
	}
	out := &unstructured.Unstructured{}
	if err := yaml.NewYAMLOrJSONDecoder(f, 4096).Decode(out); err != nil {
		return nil, err
	}
	return out, nil
}

func installWebhookOperatorCatalog(ctx context.Context, webhookOperatorCatalogPath string, oc *exutil.CLI) func() {
	cleanup, err := apply(oc, webhookOperatorCatalogPath)
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

func installWebhookOperatorExtension(ctx context.Context, webhookOperatorClusterExtPath string, oc *exutil.CLI) func() {
	cleanup, err := apply(oc, webhookOperatorClusterExtPath)
	o.Expect(err).NotTo(o.HaveOccurred())

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

func apply(oc *exutil.CLI, filePath string) (func(), error) {
	g.By(fmt.Sprintf("applying the necessary %q resources", filePath))
	err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", filePath).Execute()
	return func() {
		g.By(fmt.Sprintf("cleaning the necessary %q resources", filePath))
		err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", filePath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}, err
}
