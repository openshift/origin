package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

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
	webhookTestBaseDir              = exutil.FixturePath("testdata", "olmv1", "webhook-support")
	webhookOperatorInstallNamespace = "webhook-operator"
)

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLMWebhookProviderOpenshiftServiceCA][Skipped:Disconnected][Serial] OLMv1 operator with webhooks", g.Ordered, g.Serial, func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("openshift-operator-controller-webhooks")

	var cleanup func()

	g.BeforeEach(func(ctx g.SpecContext) {
		cleanup = setupWebhookOperator(ctx, oc)
	})

	g.AfterEach(func() {
		if cleanup != nil {
			cleanup()
		}
		if g.CurrentSpecReport().Failed() {
			exutil.DumpPodLogsStartingWith("", oc)
		}
	})

	g.It("should have a working validating webhook", func(ctx g.SpecContext) {
		g.By("creating a webhook test resource that will be rejected by the validating webhook")
		resource := getWebhookOperatorResource("validating-webhook-test", webhookOperatorInstallNamespace, false)

		// even if the resource gets created, it will be cleaned up when test gets torn down and the
		// webhook-operator namespace gets deleted
		o.Eventually(func(cg o.Gomega) {
			_, err := applyRenderedManifests(oc, resource)
			cg.Expect(err).To(o.HaveOccurred())
			cg.Expect(err.Error()).To(o.ContainSubstring("Invalid value: false: Spec.Valid must be true"))
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).Should(o.Succeed())
	})

	g.It("should have a working mutating webhook", func(ctx g.SpecContext) {
		g.By("creating a valid webhook test resource")
		var (
			mutatingWebhookResourceName = "mutating-webhook-test"
			resource                    = getWebhookOperatorResource(mutatingWebhookResourceName, webhookOperatorInstallNamespace, true)
		)

		o.Eventually(func(cg o.Gomega) {
			cleanup, err := applyRenderedManifests(oc, resource)
			cg.Expect(err).NotTo(o.HaveOccurred())
			// clean up fn will only be scheduled if there are no errors
			g.DeferCleanup(cleanup)
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).Should(o.Succeed())

		cfg, err := clientcmd.BuildConfigFromFlags("", exutil.KubeConfigPath())
		o.Expect(err).ToNot(o.HaveOccurred())
		c, err := dynamic.NewForConfig(cfg)
		o.Expect(err).ToNot(o.HaveOccurred())

		v1WebhookTestClient := c.Resource(schema.GroupVersionResource{
			Group:    "webhook.operators.coreos.io",
			Version:  "v1",
			Resource: "webhooktests",
		}).Namespace(webhookOperatorInstallNamespace)

		g.By("getting the created resource in v1 schema")
		obj, err := v1WebhookTestClient.Get(ctx, mutatingWebhookResourceName, metav1.GetOptions{})
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
		var (
			conversionWebhookResourceName = "conversion-webhook-test"
			resource                      = getWebhookOperatorResource(conversionWebhookResourceName, webhookOperatorInstallNamespace, true)
		)

		o.Eventually(func(cg o.Gomega) {
			cleanup, err := applyRenderedManifests(oc, resource)
			cg.Expect(err).NotTo(o.HaveOccurred())
			// clean up fn will only be scheduled if there are no errors
			g.DeferCleanup(cleanup)
		}).WithTimeout(10 * time.Second).WithPolling(250 * time.Millisecond).Should(o.Succeed())

		cfg, err := clientcmd.BuildConfigFromFlags("", exutil.KubeConfigPath())
		o.Expect(err).ToNot(o.HaveOccurred())
		c, err := dynamic.NewForConfig(cfg)
		o.Expect(err).ToNot(o.HaveOccurred())

		v2WebhookTestClient := c.Resource(schema.GroupVersionResource{
			Group:    "webhook.operators.coreos.io",
			Version:  "v2",
			Resource: "webhooktests",
		}).Namespace(webhookOperatorInstallNamespace)

		g.By("getting the created resource in v2 schema")
		obj, err := v2WebhookTestClient.Get(ctx, conversionWebhookResourceName, metav1.GetOptions{})
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
		resource := getWebhookOperatorResource("some-resource", webhookOperatorInstallNamespace, true)

		// delete tls secret
		g.By("deleting the openshift-service-ca signing-key secret")
		err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("secret", openshiftServiceCASigningKeySecretName, "--namespace", openshiftServiceCANamespace).Execute()
		o.Expect(err).ToNot(o.HaveOccurred())

		// even with the secret deleted the webhook should still be responsive
		g.By("checking webhook is responsive through cert rotation")
		o.Eventually(func(g o.Gomega) {
			cleanup, err := applyRenderedManifests(oc, resource)
			g.Expect(err).ToNot(o.HaveOccurred())
			cleanup()
		}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).Should(o.Succeed())
	})

	g.It("should be tolerant to tls secret deletion", func(ctx g.SpecContext) {
		resource := getWebhookOperatorResource("some-resource", webhookOperatorInstallNamespace, true)
		certificateSecretName := "webhook-operator-webhook-service-cert"

		g.By("ensuring secret exists")
		o.Eventually(func(g o.Gomega) {
			err := oc.AsAdmin().WithoutNamespace().Run("get").Args("secret", certificateSecretName, "-n", webhookOperatorInstallNamespace).Execute()
			o.Expect(err).ToNot(o.HaveOccurred())
		}).Should(o.Succeed())

		// delete tls secret
		g.By("checking webhook is responsive through secret recreation")
		err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("secret", certificateSecretName, "-n", webhookOperatorInstallNamespace).Execute()
		o.Expect(err).ToNot(o.HaveOccurred())

		// even with the secret deleted the webhook should still be responsive
		o.Eventually(func(g o.Gomega) {
			cleanup, err := applyRenderedManifests(oc, resource)
			g.Expect(err).ToNot(o.HaveOccurred())
			cleanup()
		}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(o.Succeed())
	})
})

// setupWebhookOperator sets up the webhook operator catalog and installation for a test
func setupWebhookOperator(ctx g.SpecContext, oc *exutil.CLI) func() {
	g.By("checking olmv1 capability is enabled")
	checkFeatureCapability(oc)

	g.By("creating the webhook-operator catalog")
	manifest, err := os.ReadFile(filepath.Join(webhookTestBaseDir, "webhook-operator-catalog.yaml"))
	o.Expect(err).NotTo(o.HaveOccurred())

	catalogCleanup, err := applyRenderedManifests(oc, string(manifest))
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("waiting for the webhook-operator catalog to be serving")
	var lastReason string
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			isDone, err, status := checkCatalogServing(oc, "webhook-operator-catalog")
			if lastReason != status {
				g.GinkgoLogr.Info(fmt.Sprintf("waitForWebhookOperatorCatalogServing: %q", status))
				lastReason = status
			}
			return isDone, err
		})
	o.Expect(lastReason).To(o.BeEmpty())
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("installing the webhook operator")
	extManifests, err := getWebhookClusterExtensionInstallManifests(webhookOperatorInstallNamespace)
	o.Expect(err).NotTo(o.HaveOccurred())

	operatorCleanup, err := applyRenderedManifests(oc, extManifests)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("waiting for the webhook operator to be installed")
	lastReason = ""
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			isDone, err, status := checkOperatorInstalled(oc, "webhook-operator")
			if lastReason != status {
				g.GinkgoLogr.Info(fmt.Sprintf("waitForWebhookOperatorInstalled: %q", status))
				lastReason = status
			}
			return isDone, err
		})
	o.Expect(lastReason).To(o.BeEmpty())
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("waiting for the webhook operator deployment to be Available")
	err = oc.AsAdmin().WithoutNamespace().Run("wait").Args("--for=condition=Available", "-n", webhookOperatorInstallNamespace, "deployments/webhook-operator-webhook").Execute()
	o.Expect(err).NotTo(o.HaveOccurred())

	// Return a cleanup function that cleans up both operator and catalog
	return func() {
		operatorCleanup()
		catalogCleanup()
	}
}

func checkCatalogServing(oc *exutil.CLI, catalogName string) (bool, error, string) {
	cmdArgs := []string{
		"clustercatalogs.olm.operatorframework.io",
		catalogName,
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

func checkOperatorInstalled(oc *exutil.CLI, extName string) (bool, error, string) {
	cmdArgs := []string{
		"clusterextensions.olm.operatorframework.io",
		extName,
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

func applyRenderedManifests(oc *exutil.CLI, manifests string) (func(), error) {
	g.By(fmt.Sprintf("applying manifests %q", manifests))
	err := oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", "-").InputString(manifests).Execute()
	return func() {
		err := oc.AsAdmin().WithoutNamespace().Run("delete").Args("-f", "-").InputString(manifests).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	}, err
}

func getWebhookClusterExtensionInstallManifests(installNamespace string) (string, error) {
	bytes, err := os.ReadFile(filepath.Join(webhookTestBaseDir, "webhook-operator.yaml"))
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(string(bytes), "{NAMESPACE}", installNamespace), nil
}

func getWebhookOperatorResource(name string, namespace string, valid bool) string {
	return fmt.Sprintf(`apiVersion: webhook.operators.coreos.io/v1
kind: WebhookTest
metadata:
  name: %s
  namespace: %s
spec:
  valid: %t
`, name, namespace, valid)
}
