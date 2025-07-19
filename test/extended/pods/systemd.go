package pods

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	"github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("[sig-node][Late]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("no-systemd-timeouts")

	g.It("should not have pod creation failures due to systemd timeouts", func() {
		kubeClient := oc.AdminKubeClient()

		events, err := kubeClient.CoreV1().Events("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			g.Fail(fmt.Sprintf("Unexpected error: %v", err))
		}

		timeoutStrings := []string{}
		errorMsg := "Timed out while waiting for StartTransientUnit"
		for _, event := range events.Items {
			if !strings.Contains(event.Message, errorMsg) {
				continue
			}
			timeoutString := fmt.Sprintf("systemd timed out for pod %v/%v", event.InvolvedObject.Namespace, event.InvolvedObject.Name)
			timeoutStrings = append(timeoutStrings, timeoutString)
		}
		result.Flakef("%s", strings.Join(timeoutStrings, "\n"))
	})
})
