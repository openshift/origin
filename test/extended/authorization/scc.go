package authorization

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:SCC][Early]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("working-scc-during-install")

	g.It("should not have pod creation failures during install", func() {
		kubeClient := oc.AdminKubeClient()

		events, err := kubeClient.CoreV1().Events("").List(context.TODO(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		suiteStartTime := exutil.SuiteStartTime()
		var preTestDenialStrings, duringTestDenialStrings []string
		for _, event := range events.Items {
			if !strings.Contains(event.Message, "unable to validate against any security context constraint") {
				continue
			}
			// TODO if we need more details, this is a good guess.
			//eventBytes, err := json.Marshal(event)
			//if err != nil {
			//	e2e.Logf("%v", spew.Sdump(event))
			//} else {
			//	e2e.Logf("%v", string(eventBytes))
			//}
			// try with a short summary we can actually read first
			denialString := fmt.Sprintf("%v for %v.%v/%v -n %v happened %d times", event.Message, event.InvolvedObject.Kind, event.InvolvedObject.APIVersion, event.InvolvedObject.Name, event.InvolvedObject.Namespace, event.Count)
			if event.EventTime.Time.Before(suiteStartTime) {
				preTestDenialStrings = append(preTestDenialStrings, denialString)
			} else {
				duringTestDenialStrings = append(duringTestDenialStrings, denialString)
			}
		}

		if numFailingPods := len(preTestDenialStrings); numFailingPods > 0 {
			failMessage := fmt.Sprintf("%d pods failed before test on SCC errors\n%s\n", numFailingPods, strings.Join(preTestDenialStrings, "\n"))
			result.Flakef(failMessage)
		}
		if numFailingPods := len(duringTestDenialStrings); numFailingPods > 0 {
			failMessage := fmt.Sprintf("%d pods failed during test on SCC errors\n%s\n", numFailingPods, strings.Join(duringTestDenialStrings, "\n"))
			g.Fail(failMessage)
		}
	})
})
