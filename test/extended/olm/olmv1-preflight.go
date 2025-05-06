package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLMPreflightPermissionChecks][Skipped:Disconnected] OLMv1 operator preflight checks", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("openshift-operator-controller")

	g.BeforeEach(func() {
		exutil.PreTestDump()
	})

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			exutil.DumpPodLogsStartingWith("", oc)
		}
	})

	g.It("should report error when {services} are not specified", func(ctx g.SpecContext) {
		runNegativePreflightTest(ctx, oc, 1)
	})

	g.It("should report error when {create} verb is not specified", func(ctx g.SpecContext) {
		runNegativePreflightTest(ctx, oc, 2)
	})

	g.It("should report error when {ClusterRoleBindings} are not specified", func(ctx g.SpecContext) {
		runNegativePreflightTest(ctx, oc, 3)
	})

	g.It("should report error when {ConfigMap:resourceNames} are not all specified", func(ctx g.SpecContext) {
		runNegativePreflightTest(ctx, oc, 4)
	})

	g.It("should report error when {clusterextension/finalizer} is not specified", func(ctx g.SpecContext) {
		runNegativePreflightTest(ctx, oc, 5)
	})

	g.It("should report error when {escalate, bind} is not specified", func(ctx g.SpecContext) {
		runNegativePreflightTest(ctx, oc, 6)
	})
})

func runNegativePreflightTest(ctx g.SpecContext, oc *exutil.CLI, iteration int) {
	checkFeatureCapability(oc)

	baseDir := exutil.FixturePath("testdata", "olmv1")
	crFile := filepath.Join(baseDir, fmt.Sprintf("install-pipeline-operator-%d.yaml", iteration))
	ceFile := filepath.Join(baseDir, "install-pipeline-operator-base.yaml")

	g.By(fmt.Sprintf("applying %s", crFile))
	cleanupCr, unique := applyPreflightFile(oc, "", crFile)
	g.DeferCleanup(cleanupCr)

	g.By(fmt.Sprintf("applying %s", ceFile))
	cleanupCe, _ := applyPreflightFile(oc, unique, ceFile)
	g.DeferCleanup(cleanupCe)

	ceName := "install-test-ce-" + unique

	g.By("waiting for the ClusterExtention to report failure")
	var lastReason string
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			b, err, s := waitForPreflightFailure(oc, ceName)
			if lastReason != s {
				g.GinkgoLogr.Info(fmt.Sprintf("waitForPreflightFailure: %q", s))
				lastReason = s
			}
			return b, err
		})
	o.Expect(lastReason).To(o.BeEmpty())
	o.Expect(err).NotTo(o.HaveOccurred())
}

func applyPreflightFile(oc *exutil.CLI, unique, file string) (func(), string) {
	// packageName and version are specified in the files and do not change
	return applyResourceFile(oc, "", "", unique, file)
}

func waitForPreflightFailure(oc *exutil.CLI, ceName string) (bool, error, string) {
	var conditions []metav1.Condition
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("clusterextensions.olm.operatorframework.io", ceName, "-o=jsonpath={.status.conditions}").Output()
	if err != nil {
		return false, err, ""
	}
	// no data yet, so try again
	if output == "" {
		return false, nil, "no output"
	}
	if err := json.Unmarshal([]byte(output), &conditions); err != nil {
		return false, fmt.Errorf("error in json.Unmarshal(%v): %v", output, err), ""
	}
	c := meta.FindStatusCondition(conditions, typeProgressing)
	if c == nil {
		return false, nil, fmt.Sprintf("condition not present: %q", typeProgressing)
	}
	if c.Status != metav1.ConditionTrue {
		return false, nil, fmt.Sprintf("expected status to be %q: %+v", metav1.ConditionTrue, c)
	}

	messagePrefix := "pre-authorization failed:"
	if !strings.HasPrefix(c.Message, messagePrefix) {
		return false, nil, fmt.Sprintf("expected message to contain %q: %+v", messagePrefix, c)
	}

	messageContains := "service account requires the following permissions to manage cluster extension:"
	if !strings.Contains(c.Message, messageContains) {
		return false, nil, fmt.Sprintf("expected message to contain %q: %+v", messageContains, c)
	}

	c = meta.FindStatusCondition(conditions, typeInstalled)
	if c == nil {
		return false, nil, fmt.Sprintf("condition not present: %q", typeInstalled)
	}
	if c.Status != metav1.ConditionFalse {
		return false, nil, fmt.Sprintf("expected status to be %q: %+v", metav1.ConditionFalse, c)
	}
	return true, nil, ""
}
