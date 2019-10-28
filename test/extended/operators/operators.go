package operators

import (
	"bytes"
	"fmt"
	"os"
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

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
)

const (
	operatorWait = 1 * time.Minute
	cvoWait      = 5 * time.Minute
)

var _ = g.Describe("[Feature:Platform][Smoke] Managed cluster should", func() {
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
			obj, err := cvc.Get("version", metav1.GetOptions{})
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
		available := make(map[string]struct{})
		g.By(fmt.Sprintf("waiting for all cluster operators to be stable at the same time"))
		coc := dc.Resource(schema.GroupVersionResource{Group: "config.openshift.io", Resource: "clusteroperators", Version: "v1"})
		lastErr = nil
		var lastCOs []objx.Map
		wait.PollImmediate(time.Second, operatorWait, func() (bool, error) {
			obj, err := coc.List(metav1.ListOptions{})
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

			var unavailable []objx.Map
			var unavailableNames []string
			for _, co := range items {
				if condition(co, "Available").Get("status").String() != "True" {
					ns := co.Get("metadata.namespace").String()
					name := co.Get("metadata.name").String()
					unavailableNames = append(unavailableNames, fmt.Sprintf("%s/%s", ns, name))
					unavailable = append(unavailable, co)
					break
				}
				if condition(co, "Progressing").Get("status").String() != "False" {
					ns := co.Get("metadata.namespace").String()
					name := co.Get("metadata.name").String()
					unavailableNames = append(unavailableNames, fmt.Sprintf("%s/%s", ns, name))
					unavailable = append(unavailable, co)
					break
				}
				if condition(co, "Failing").Get("status").String() != "False" {
					ns := co.Get("metadata.namespace").String()
					name := co.Get("metadata.name").String()
					unavailableNames = append(unavailableNames, fmt.Sprintf("%s/%s", ns, name))
					unavailable = append(unavailable, co)
					break
				}
			}
			if len(unavailable) > 0 {
				e2e.Logf("Operators still doing work: %s", strings.Join(unavailableNames, ", "))
				return false, nil
			}
			return true, nil
		})

		o.Expect(lastErr).NotTo(o.HaveOccurred())
		var unavailable []string
		buf := &bytes.Buffer{}
		w := tabwriter.NewWriter(buf, 0, 4, 1, ' ', 0)
		fmt.Fprintf(w, "NAMESPACE\tNAME\tPROGRESSING\tAVAILABLE\tVERSION\tMESSAGE\n")
		for _, co := range lastCOs {
			ns := co.Get("metadata.namespace").String()
			name := co.Get("metadata.name").String()
			if condition(co, "Available").Get("status").String() != "True" {
				unavailable = append(unavailable, fmt.Sprintf("%s/%s", ns, name))
			} else {
				available[fmt.Sprintf("%s/%s", ns, name)] = struct{}{}
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				ns,
				name,
				condition(co, "Progressing").Get("status").String(),
				condition(co, "Available").Get("status").String(),
				co.Get("status.version").String(),
				condition(co, "Failing").Get("message").String(),
			)
		}
		w.Flush()
		e2e.Logf("ClusterOperators:\n%s", buf.String())

		if len(unavailable) > 0 {
			e2e.Failf("Some cluster operators never became available %s", strings.Join(unavailable, ", "))
		}
		// Check at least one core operator is available
		if len(available) == 0 {
			e2e.Failf("There must be at least one cluster operator")
		}
	})
})

var _ = g.Describe("[Feature:Platform] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("have operators on the cluster version", func() {
		if len(os.Getenv("TEST_UNSUPPORTED_ALLOW_VERSION_SKEW")) > 0 {
			e2e.Skipf("Test is disabled to allow cluster components to have different versions")
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
		cv, err := c.ConfigV1().ClusterVersions().Get("version", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		coList, err := c.ConfigV1().ClusterOperators().List(metav1.ListOptions{})
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
		_, err := c.Get("openshift-cluster-version", metav1.GetOptions{})
		if err == nil {
			return true, nil
		}
		if errors.IsNotFound(err) {
			e2e.Skipf("The cluster is not managed by a cluster-version operator")
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
