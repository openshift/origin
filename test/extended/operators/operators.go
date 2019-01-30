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

	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/test/extended/prometheus"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	operatorWait = 1 * time.Minute
	cvoWait      = 5 * time.Minute

	// pods whose metrics show a larger ratio of requests per
	// second than maxQPSAllowed are considered "unhealthy".
	maxQPSAllowed = 7.0 // ideal value: 1.5
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
})

var _ = g.Describe("[Feature:Platform] Managed cluster", func() {
	defer g.GinkgoRecover()
	var (
		oc               = exutil.NewCLIWithoutNamespace("operators")
		url, bearerToken string
	)
	g.BeforeEach(func() {
		var ok bool
		url, bearerToken, ok = prometheus.LocatePrometheus(oc)
		if !ok {
			e2e.Skipf("Prometheus could not be located on this cluster, skipping prometheus test")
		}
	})

	g.It("should iterate through operator pods and detect higher-than-normal queries per second", func() {
		if !prometheus.HasPullSecret(oc.AdminKubeClient(), "cloud.openshift.com") {
			e2e.Skipf("Telemetry is disabled")
		}
		oc.SetupProject()
		ns := oc.Namespace()

		execPodName := e2e.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod", func(pod *v1.Pod) { pod.Spec.Containers[0].Image = "centos:7" })
		defer func() { oc.AdminKubeClient().Core().Pods(ns).Delete(execPodName, metav1.NewDeleteOptions(1)) }()

		client, err := configclient.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		operators, err := client.ConfigV1().ClusterOperators().List(metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// TODO: standardize operator / operator client names
		// names of exceptions for operators with client names that deviate slightly from what is expected
		knownNameConversions := map[string]string{
			"kube-controller-manager":               "cluster-kube-controller-manager-operator",
			"openshift-apiserver":                   "cluster-openshift-apiserver-operator",
			"openshift-cluster-samples-operator":    "cluster-samples-operator",
			"openshift-controller-manager-operator": "cluster-openshift-controller-manager-operator",
		}

		// names of operators without corresponding metrics
		knownMissingOperators := map[string]bool{
			"cluster-monitoring-operator": true,
		}

		// obtain cluster operator client names
		operatorClientNames := []string{}
		for _, co := range operators.Items {
			clientName := co.Name
			if knownMissingOperators[clientName] {
				continue
			}

			if cName, ok := knownNameConversions[clientName]; ok {
				clientName = cName
			} else if strings.HasPrefix(clientName, "openshift-") {
				clientName = strings.Replace(clientName, "openshift-", "cluster-", 1)
			}
			operatorClientNames = append(operatorClientNames, clientName)
		}

		// detect rate of requests for a given operator over a timespan of 4 mins
		metricFormat := `sum(rate(apiserver_request_count{client="%s/v0.0.0 (linux/amd64) kubernetes/$Format"}[4m]))`

		tests := map[string][]prometheus.MetricTest{}
		for _, c := range operatorClientNames {
			tests[fmt.Sprintf(metricFormat, c)] = []prometheus.MetricTest{{GreaterThanEqual: false, Value: maxQPSAllowed}}
		}

		prometheus.RunQueries(tests, oc, ns, execPodName, url, bearerToken)
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
