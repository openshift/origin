package nodes

import (
	"context"
	"fmt"
	g "github.com/onsi/ginkgo/v2"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"strings"
)

var _ = g.Describe("[sig-node][Late]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("no-unready-nodes")
	cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().
		       Get(context.Background(), "version", metav1.GetOptions{})

	if err != nil {
		e2e.Failf("Unable to determine cluster install completionTime :" + cv.Name)
	}

	g.It("nodes should not go unready after cluster install is complete", func() {

		var failures []string
		events, err := oc.AdminKubeClient().CoreV1().Events("").List(context.TODO(), metav1.ListOptions{})

		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		nodesWentUnready := make(map[string][]string)

		for _, event := range events.Items {
			if strings.Contains(event.Reason, "NodeNotReady") &&
				cv.Status.History[0].CompletionTime.Time.Before(event.CreationTimestamp.Time) {

				failureMessage := fmt.Sprintf("Node went unready at %s with message \"%s\" after " +
					"cluster install was complete at %s",
					event.CreationTimestamp, event.Message, cv.Status.History[0].CompletionTime)
				nodesWentUnready[event.Name] = append(nodesWentUnready[event.Name], failureMessage)
			}
		}

		for unready := range nodesWentUnready {
			if len(unready) > 0 {
				failures = append(failures, fmt.Sprintf("Node went unready: %s", unready))
			}
		}

		if len(failures) > 0 {
			e2e.Failf(strings.Join(failures, "\n"))
		}

	})
})