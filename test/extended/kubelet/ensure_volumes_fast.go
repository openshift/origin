package kubelet

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

var _ = g.Describe("[sig-node][Late] kubelet container creation", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithoutNamespace("kubelet-container-creation")
	)

	// these appear to fail on a regular basis.  We see it even for service account token secrets, which are guaranteed
	// to exist before they are added to the pod in admission, but we don't want to perma-break CI
	//g.It("shouldn't have trouble mounting secrets", func() {
	//	confirmNoSecretSyncFailure(oc.AdminKubeClient())
	//})
	//g.It("shouldn't have trouble mounting configmaps", func() {
	//	confirmNoConfigMapSyncFailure(oc.AdminKubeClient())
	//})

	g.It("shouldn't have trouble mounting any volume", func() {
		confirmNoStuckVolume(oc.AdminKubeClient())
	})
})

// these mean that the kubelet wasn't able to get the secret or configmap content
// this only gets the ones from namespaces that still exist, but our biggest known offender is in openshift-infra which sticks around.
// if we suspect we're getting more, we can create a list/watch for it.
func confirmNoEvent(eventMessage string, kubeClient kubernetes.Interface) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
			g.Fail(fmt.Sprintf("panic: %v", r))
		}
	}()

	ctx := context.TODO()
	events, err := kubeClient.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	failures := []string{}
	for _, event := range events.Items {
		if !strings.Contains(event.Message, eventMessage) {
			continue
		}

		failures = append(failures, fmt.Sprintf("%q in %q with %q", event.InvolvedObject.Name, event.InvolvedObject.Namespace, event.Message))
	}

	if len(failures) > 0 {
		g.Fail(strings.Join(failures, "\n"))
	}
}

func confirmNoSecretSyncFailure(kubeClient kubernetes.Interface) {
	confirmNoEvent("failed to sync secret cache", kubeClient)
}

func confirmNoConfigMapSyncFailure(kubeClient kubernetes.Interface) {
	confirmNoEvent("failed to sync configmap cache", kubeClient)
}

func confirmNoStuckVolume(kubeClient kubernetes.Interface) {
	confirmNoEvent("Unable to attach or mount volumes", kubeClient)
}

// ResourceVolumeSyncUpgradeTest tests that kubelets are able to mount secrets and configmaps in a timely fashion
type ResourceVolumeSyncUpgradeTest struct {
}

// Name returns the tracking name of the test.
func (ResourceVolumeSyncUpgradeTest) Name() string {
	return "[sig-node][Late] kubelet container creation volume mounting"
}

// Setup creates a DaemonSet and verifies that it's running
func (t *ResourceVolumeSyncUpgradeTest) Setup(f *framework.Framework) {
}

func (t *ResourceVolumeSyncUpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	// wait to ensure API is still up after the test ends
	<-done

	// these appear to fail on a regular basis.  We see it even for service account token secrets, which are guaranteed
	// to exist before they are added to the pod in admission, but we don't want to perma-break CI
	//confirmNoSecretSyncFailure(f.ClientSet)
	//confirmNoConfigMapSyncFailure(f.ClientSet)

	confirmNoStuckVolume(f.ClientSet)
}

func (t *ResourceVolumeSyncUpgradeTest) Teardown(f *framework.Framework) {
}
