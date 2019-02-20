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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	syncStatusWait = 2 * time.Minute
)

var _ = g.Describe("[Feature:Platform][Disruptive] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("reconcile cluster operator status", func() {
		cfg, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		c, err := e2e.LoadClientset()
		o.Expect(err).NotTo(o.HaveOccurred())
		dc, err := dynamic.NewForConfig(cfg)
		o.Expect(err).NotTo(o.HaveOccurred())

		// presence of the CVO namespace gates this test
		g.By("checking for the cluster version operator")
		skipUnlessCVO(c.CoreV1().Namespaces())

		// gate on all clusteroperators being ready
		available := make(map[string]struct{})
		g.By(fmt.Sprintf("waiting for all cluster operators to be stable at the same time"))
		coc := dc.Resource(schema.GroupVersionResource{Group: "config.openshift.io", Resource: "clusteroperators", Version: "v1"})
		var lastErr error
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

		g.By("deleting all the cluster operator resources")
		expectedClusterOperators := sets.NewString()
		for _, co := range lastCOs {
			name := co.Get("metadata.name").String()
			expectedClusterOperators.Insert(name)
			if err := coc.Delete(name, nil); err != nil {
				e2e.Failf("Unable to delete cluster operator resource: %s with err: %v", name, err)
			}
		}

		g.By("waiting for all previous cluster operators to report available again")
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

			actualClusterOperators := sets.NewString()
			var unavailable []objx.Map
			var unavailableNames []string
			for _, co := range items {
				ns := co.Get("metadata.namespace").String()
				name := co.Get("metadata.name").String()
				actualClusterOperators.Insert(name)
				if condition(co, "Available").Get("status").String() != "True" {
					unavailableNames = append(unavailableNames, fmt.Sprintf("%s/%s", ns, name))
					unavailable = append(unavailable, co)
					break
				}
				if condition(co, "Progressing").Get("status").String() != "False" {
					unavailableNames = append(unavailableNames, fmt.Sprintf("%s/%s", ns, name))
					unavailable = append(unavailable, co)
					break
				}
				if condition(co, "Failing").Get("status").String() != "False" {
					unavailableNames = append(unavailableNames, fmt.Sprintf("%s/%s", ns, name))
					unavailable = append(unavailable, co)
					break
				}
			}
			if len(unavailable) > 0 {
				e2e.Logf("Operators still doing work: %s", strings.Join(unavailableNames, ", "))
				return false, nil
			}
			if !actualClusterOperators.Equal(expectedClusterOperators) {
				e2e.Logf("Operators missing reported status: %v", expectedClusterOperators.Difference(actualClusterOperators))
				return false, nil
			}
			return true, nil
		})

		var unavailable []string
		actualClusterOperators := sets.NewString()
		buf := &bytes.Buffer{}
		w := tabwriter.NewWriter(buf, 0, 4, 1, ' ', 0)
		fmt.Fprintf(w, "NAMESPACE\tNAME\tPROGRESSING\tAVAILABLE\tVERSION\tMESSAGE\n")
		for _, co := range lastCOs {
			ns := co.Get("metadata.namespace").String()
			name := co.Get("metadata.name").String()
			actualClusterOperators.Insert(name)
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

		if !actualClusterOperators.Equal(expectedClusterOperators) {
			e2e.Failf("Operators missing reported status: %v", expectedClusterOperators.Difference(actualClusterOperators))
		}

		if len(unavailable) > 0 {
			e2e.Failf("Some cluster operators never became available %s", strings.Join(unavailable, ", "))
		}
		// Check at least one core operator is available
		if len(available) == 0 {
			e2e.Failf("There must be at least one cluster operator")
		}
	})
})
