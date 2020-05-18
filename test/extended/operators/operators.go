package operators

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/stretchr/objx"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
)

const (
	operatorWait = 1 * time.Minute
	cvoWait      = 5 * time.Minute
)

var _ = g.Describe("[sig-arch][Early] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("start all core operators", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		// presence of the CVO namespace gates this test
		g.By("checking for the cluster version operator")
		skipUnlessCVO(c.CoreV1().Namespaces())

		g.By("waiting for the cluster version to be applied")
		cvc := dc.Resource(schema.GroupVersionResource{Group: "config.openshift.io", Resource: "clusterversions", Version: "v1"})
		var lastErr error
		var lastCV objx.Map
		if err := wait.PollImmediate(3*time.Second, cvoWait, func() (bool, error) {
			obj, err := cvc.Get(context.Background(), "version", metav1.GetOptions{})
			if err != nil {
				lastErr = err
				e2e.Logf("Unable to check for cluster version: %v", err)
				return false, nil
			}
			cv := objx.Map(obj.UnstructuredContent())
			lastErr = nil
			lastCV = cv
			if cond := condition(cv, "Progressing"); cond.Get("status").String() != "False" {
				e2e.Logf("ClusterVersion is still progressing: %s", cond.Get("message").String())
				return false, nil
			}
			if cond := condition(cv, "Available"); cond.Get("status").String() != "True" {
				e2e.Logf("ClusterVersion is not available: %s", cond.Get("message").String())
				return false, nil
			}
			e2e.Logf("ClusterVersion available: %s", condition(cv, "Progressing").Get("message").String())
			return true, nil
		}); err != nil {
			o.Expect(lastErr).NotTo(o.HaveOccurred())
			e2e.Logf("Last cluster version seen: %s", lastCV)
			if msg := condition(lastCV, "Failing").Get("message").String(); len(msg) > 0 {
				e2e.Logf("ClusterVersion is reporting a failure: %s", msg)
			}
			e2e.Failf("ClusterVersion never became available: %s", condition(lastCV, "Progressing").Get("message").String())
		}

		// gate on all clusteroperators being ready
		g.By(fmt.Sprintf("waiting for all cluster operators to be stable at the same time"))
		coc := dc.Resource(schema.GroupVersionResource{Group: "config.openshift.io", Resource: "clusteroperators", Version: "v1"})
		lastErr = nil
		var lastCOs []objx.Map
		wait.PollImmediate(time.Second, operatorWait, func() (bool, error) {
			obj, err := coc.List(context.Background(), metav1.ListOptions{})
			if err != nil {
				lastErr = err
				e2e.Logf("Unable to check for cluster operators: %v", err)
				return false, nil
			}
			cv := objx.Map(obj.UnstructuredContent())
			lastErr = nil
			items := objects(cv.Get("items"))
			lastCOs = items

			if len(items) == 0 {
				return false, nil
			}

			var unready []string
			for _, co := range items {
				badConditions, missingTypes := surprisingConditions(co)
				if len(badConditions) > 0 || len(missingTypes) > 0 {
					unready = append(unready, co.Get("metadata.name").String())
				}
			}
			if len(unready) > 0 {
				sort.Strings(unready)
				e2e.Logf("Operators still unready: %s", strings.Join(unready, ", "))
				return false, nil
			}
			return true, nil
		})

		o.Expect(lastErr).NotTo(o.HaveOccurred())
		ready := 0
		var unready []string
		buf := &bytes.Buffer{}
		w := tabwriter.NewWriter(buf, 0, 4, 1, ' ', 0)
		fmt.Fprintf(w, "NAME\tTYPE\tSTATUS\tREASON\tMESSAGE\n")
		for _, co := range lastCOs {
			name := co.Get("metadata.name").String()
			badConditions, missingTypes := surprisingConditions(co)
			if len(badConditions) > 0 {
				worstCondition := badConditions[0]
				unready = append(unready, fmt.Sprintf("%s (%s=%s %s: %s)",
					name,
					worstCondition.Type,
					worstCondition.Status,
					worstCondition.Reason,
					worstCondition.Message,
				))
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					name,
					worstCondition.Type,
					worstCondition.Status,
					worstCondition.Reason,
					worstCondition.Message,
				)
			} else if len(missingTypes) > 0 {
				missingTypeStrings := make([]string, 0, len(missingTypes))
				for _, missingType := range missingTypes {
					missingTypeStrings = append(missingTypeStrings, string(missingType))
				}
				unready = append(unready, fmt.Sprintf("%s (missing: %s)", name, strings.Join(missingTypeStrings, ", ")))
			} else {
				ready++
			}
		}
		w.Flush()
		e2e.Logf("ClusterOperators:\n%s", buf.String())

		if len(unready) > 0 {
			sort.Strings(unready)
			e2e.Failf("Some cluster operators never became ready: %s", strings.Join(unready, ", "))
		}
		// Check at least one core operator is ready
		if ready == 0 {
			e2e.Failf("There must be at least one cluster operator")
		}
	})
})

var _ = g.Describe("[sig-arch] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("have operators on the cluster version", func() {
		if len(os.Getenv("TEST_UNSUPPORTED_ALLOW_VERSION_SKEW")) > 0 {
			e2eskipper.Skipf("Test is disabled to allow cluster components to have different versions")
		}
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c := configclient.NewForConfigOrDie(cfg)
		coreclient, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())

		// presence of the CVO namespace gates this test
		g.By("checking for the cluster version operator")
		skipUnlessCVO(coreclient.CoreV1().Namespaces())

		// we need to get the list of versions
		cv, err := c.ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		coList, err := c.ConfigV1().ClusterOperators().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(coList.Items).NotTo(o.BeEmpty())

		g.By("all cluster operators report an operator version in the first position equal to the cluster version")
		for _, co := range coList.Items {
			msg := fmt.Sprintf("unexpected operator status versions %s:\n%#v", co.Name, co.Status.Versions)
			o.Expect(co.Status.Versions).NotTo(o.BeEmpty(), msg)
			operator := findOperatorVersion(co.Status.Versions, "operator")
			o.Expect(operator).NotTo(o.BeNil(), msg)
			o.Expect(operator.Name).To(o.Equal("operator"), msg)
			o.Expect(operator.Version).To(o.Equal(cv.Status.Desired.Version), msg)
		}
	})
})

func skipUnlessCVO(c coreclient.NamespaceInterface) {
	err := wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		_, err := c.Get(context.Background(), "openshift-cluster-version", metav1.GetOptions{})
		if err == nil {
			return true, nil
		}
		if errors.IsNotFound(err) {
			e2eskipper.Skipf("The cluster is not managed by a cluster-version operator")
		}
		e2e.Logf("Unable to check for cluster version operator: %v", err)
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func findOperatorVersion(versions []configv1.OperandVersion, name string) *configv1.OperandVersion {
	for i := range versions {
		if versions[i].Name == name {
			return &versions[i]
		}
	}
	return nil
}

func contains(names []string, name string) bool {
	for _, s := range names {
		if s == name {
			return true
		}
	}
	return false
}

func jsonString(from objx.Map) string {
	s, _ := from.JSON()
	return s
}

func objects(from *objx.Value) []objx.Map {
	var values []objx.Map
	switch {
	case from.IsObjxMapSlice():
		return from.ObjxMapSlice()
	case from.IsInterSlice():
		for _, i := range from.InterSlice() {
			if msi, ok := i.(map[string]interface{}); ok {
				values = append(values, objx.Map(msi))
			}
		}
	}
	return values
}

func condition(cv objx.Map, condition string) objx.Map {
	for _, obj := range objects(cv.Get("status.conditions")) {
		if obj.Get("type").String() == condition {
			return obj
		}
	}
	return objx.Map(nil)
}

// surprisingConditions returns conditions with surprising statuses
// (Available=False, Degraded=True, etc.) in order of descending
// severity (e.g. Available=False is more severe than Degraded=True).
// It also returns a slice of types for which a condition entry was
// expected but not supplied on the ClusterOperator.
func surprisingConditions(co objx.Map) ([]configv1.ClusterOperatorStatusCondition, []configv1.ClusterStatusConditionType) {
	var badConditions []configv1.ClusterOperatorStatusCondition
	var missingTypes []configv1.ClusterStatusConditionType
	for _, conditionType := range []configv1.ClusterStatusConditionType{
		configv1.OperatorAvailable,
		configv1.OperatorDegraded,
	} {
		cond := condition(co, string(conditionType))
		if len(cond) == 0 {
			missingTypes = append(missingTypes, conditionType)
		} else {
			expected := configv1.ConditionFalse
			if conditionType == configv1.OperatorAvailable {
				expected = configv1.ConditionTrue
			}
			if cond.Get("status").String() != string(expected) {
				badConditions = append(badConditions, configv1.ClusterOperatorStatusCondition{
					Type:    conditionType,
					Status:  configv1.ConditionStatus(cond.Get("status").String()),
					Reason:  cond.Get("reason").String(),
					Message: cond.Get("message").String(),
				})
			}
		}
	}
	return badConditions, missingTypes
}
