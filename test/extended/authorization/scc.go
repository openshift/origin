package authorization

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blang/semver"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	"github.com/openshift/origin/test/extended/util"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-auth][Feature:SCC][Early]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("working-scc-during-install")

	g.It("should not have pod creation failures during install", func() {
		kubeClient := oc.AdminKubeClient()

		// prior to 4.10, we saw races in releases from 4.1 onwards with
		// 1. SCC type didn't exist: we moved it to a CRD
		// 2. SCC didn't exist: we moved them to rendering step
		// 3. namespace annotations didn't exist before openshift-controller-manager: cluster-policy-controller created as part of KCM pod
		// 4. SCC creation/lister was not complete: we created a "wait for up to 10s" stanza for synchronization
		// 5. namespaces were not annotated with UID ranges fast enough: created a "wait for up to 10s" stanza to allow the race to settle
		// The culmination of all these things has allowed us to be tighter post-4.10 or whatever level we backport to.
		// It's worth noting that these could result in a different SCC being assigned and that can result in different
		// file ownership, so being stable early is important.
		numFailuresForFail := 5

		events, err := kubeClient.CoreV1().Events("").List(context.TODO(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		// starting from 4.10, enforce the requirement that SCCs not fail
		config, err := framework.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())
		hasAllFixes, err := util.AllClusterVersionsAreGTE(semver.Version{Major: 4, Minor: 10}, config)
		if err != nil {
			framework.Logf("Cannot require full SCC enforcement, some versions could not be checked: %v", err)
		}
		if hasAllFixes {
			numFailuresForFail = 0
		}

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