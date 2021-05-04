package authorization

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("[sig-auth][Feature:SCC][Early]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("working-scc-during-install")

	g.It("should not have pod creation failures during install", func() {
		kubeClient := oc.AdminKubeClient()

		// we are seeting a few install-time failures, mostly around extremely early pods like CSI drivers, therefore
		// we allow a few failures here.
		numFailuresForFail := 2

		events, err := kubeClient.CoreV1().Events("").List(context.TODO(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		suiteStartTime := exutil.SuiteStartTime()
		var suppressPreTestFailure bool
		if t := exutil.LimitTestsToStartTime(); !t.IsZero() {
			suppressPreTestFailure = true
			if t.After(suiteStartTime) {
				suiteStartTime = t
			}
		}

		var preTestDenialStrings, duringTestDenialStrings []string

		for _, event := range events.Items {
			if !strings.Contains(event.Message, "unable to validate against any security context constraint") {
				continue
			}

			// try with a short summary we can actually read first
			denialString := fmt.Sprintf("%v for %v.%v/%v -n %v happened %d times", event.Message, event.InvolvedObject.Kind, event.InvolvedObject.APIVersion, event.InvolvedObject.Name, event.InvolvedObject.Namespace, event.Count)

			if event.EventTime.Time.Before(suiteStartTime) {
				// SCCs become accessible to serviceaccounts based on RBAC resources.  We could require that every operator
				// apply their RBAC in order with respect to their operands by checking SARs against every kube-apiserver endpoint
				// and ensuring that the "use" for an SCC comes back correctly, but that isn't very useful.
				// We don't want to delay pods for an excessive period of time, so we will catch those pods that take more
				// than five seconds to make it through SCC
				durationPodFailed := event.LastTimestamp.Sub(event.FirstTimestamp.Time)
				if durationPodFailed < 5*time.Second {
					continue
				}
				preTestDenialStrings = append(preTestDenialStrings, denialString)
			} else {
				// Tests are not allowed to not take SCC propagation time into account, and so every during test SCC failure
				// is a hard fail so that we don't allow bad tests to get checked in.
				duringTestDenialStrings = append(duringTestDenialStrings, denialString)
			}
		}

		if numFailingPods := len(preTestDenialStrings); numFailingPods > numFailuresForFail {
			failMessage := fmt.Sprintf("%d pods failed before test on SCC errors\n%s\n", numFailingPods, strings.Join(preTestDenialStrings, "\n"))
			if suppressPreTestFailure {
				result.Flakef("pre-test environment had disruption and limited this test, suppressing failure: %s", failMessage)
			} else {
				g.Fail(failMessage)
			}
		}
		if numFailingPods := len(duringTestDenialStrings); numFailingPods > 0 {
			failMessage := fmt.Sprintf("%d pods failed during test on SCC errors\n%s\n", numFailingPods, strings.Join(duringTestDenialStrings, "\n"))
			g.Fail(failMessage)
		}
	})
})
