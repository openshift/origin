package operators

import (
	"bytes"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/stretchr/objx"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	operatorWait = 1 * time.Minute
	cvoWait      = 5 * time.Minute

	// pods whose metrics show a larger ratio of requests per
	// second than maxQPSAllowed are considered "unhealthy".
	maxQPSAllowed = 1.5
)

var (
	// TODO: these exceptions should not exist. Update operators to have a better request-rate per second
	perComponentNamespaceMaxQPSAllowed = map[string]float64{
		"openshift-apiserver-operator":                            3.0,
		"openshift-kube-apiserver-operator":                       6.8,
		"openshift-kube-controller-manager-operator":              2.0,
		"openshift-cluster-kube-scheduler-operator":               1.8,
		"openshift-cluster-openshift-controller-manager-operator": 1.7,
		"openshift-kube-scheduler-operator":                       1.7,
	}
)

var _ = g.Describe("[Feature:Platform][Smoke] Managed cluster should", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLIWithoutNamespace("operators")

	g.BeforeEach(func() {
		if !locatePrometheus(oc) {
			e2e.Skipf("Prometheus could not be located on this cluster, skipping operator test")
		}
	})

	g.It("start all core operators", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		// presence of the CVO namespace gates this test
		g.By("checking for the cluster version operator")
		skipUnlessCVO(c.Core().Namespaces())

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
			payload := cv.Get("status.current.payload").String()
			if len(payload) == 0 {
				e2e.Logf("ClusterVersion has no current payload version")
				return false, nil
			}
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
		for _, group := range []string{"config.openshift.io"} {
			g.By(fmt.Sprintf("waiting for all cluster operators in %s to be available", group))
			coc := dc.Resource(schema.GroupVersionResource{Group: group, Resource: "clusteroperators", Version: "v1"})
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

				// TODO: make this an error condition once we know at least one cluster operator status is reported
				if len(items) == 0 {
					e2e.Logf("No cluster operators registered in %s", group)
					return true, nil
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
				}
				if len(unavailable) > 0 {
					e2e.Logf("Operators in group %s still unavailable: %s", group, strings.Join(unavailableNames, ", "))
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
			// TODO: make this an e2e.Failf()
			if len(unavailable) > 0 {
				e2e.Logf("Some cluster operators never became available %s", strings.Join(unavailable, ", "))
			}
		}

		// Check at least one core operator is available
		if len(available) == 0 {
			e2e.Failf("None of the required core operators are available")
		}
	})

	g.It("should iterate through operator pods and detect higher-than-normal queries per second", func() {
		podURLGetter := &portForwardURLGetter{
			Protocol:   "https",
			Host:       "localhost",
			RemotePort: "8443",
			LocalPort:  "37587",
		}

		namespaces, err := oc.AdminKubeClient().CoreV1().Namespaces().List(metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		failures := []error{}
		failedPods := []v1.Pod{}
		for _, ns := range namespaces.Items {
			// skip namespaces which do not meet "operator namespace" criteria
			if !strings.HasPrefix(ns.Name, "openshift-") || !strings.HasSuffix(ns.Name, "-operator") {
				continue
			}

			infos, err := getPodInfoForNamespace(oc.AdminKubeClient(), oc.AdminConfig(), podURLGetter, ns.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, info := range infos {
				if info.failed {
					failures = append(failures, fmt.Errorf("failed to fetch operator pod metrics for pod %q: %s", info.name, info.result))
					continue
				}
				if info.skipped {
					continue
				}

				qpsLimit := maxQPSAllowed
				if customLimit, ok := perComponentNamespaceMaxQPSAllowed[info.namespace]; ok {
					qpsLimit = customLimit
				}

				if info.qps > qpsLimit {
					failedPods = append(failedPods, *info.pod)
					failures = append(failures, fmt.Errorf("operator pod %q in namespace %q is making %v requests per second. Maximum allowed is %v requests per second", info.name, info.namespace, info.qps, maxQPSAllowed))
					continue
				}
			}

			if len(failures) > 0 {
				exutil.DumpPodLogs(failedPods, oc)
			}
			o.Expect(failures).To(o.BeEmpty())
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
