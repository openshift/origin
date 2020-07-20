package networking

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/openshift/client-go/operatorcontrolplane/clientset/versioned/typed/operatorcontrolplane/v1alpha1"
	exutil "github.com/openshift/origin/test/extended/util"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

var _ = g.Describe("[sig-network][Late] service network access from openshift-apiserver to kube-apiserver", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithoutNamespace("service-network-openshift-apiserver")
	)

	g.It("shouldn't report outage to kubernetes-service", func() {
		confirmNoKubernetesDefaultServiceNetworkOutage(oc.AdminConfig())
	})

	g.It("shouldn't report outage to kubernetes-apiserver-service", func() {
		confirmNoKubernetesServiceMonitorServiceNetworkOutage(oc.AdminConfig())
	})
})

func confirmNoKubernetesDefaultServiceNetworkOutage(clientConfig *rest.Config) {
	confirmNoServiceNetworkOutage("kubernetes-default-service", "KUBERNETES_SERVICE_HOST:KUBERNETES_SERVICE_PORT", clientConfig)
}

func confirmNoKubernetesServiceMonitorServiceNetworkOutage(clientConfig *rest.Config) {
	confirmNoServiceNetworkOutage("kubernetes-apiserver-service", "`oc -n openshift-kube-apiserver get services/apiserver`", clientConfig)
}

func confirmNoServiceNetworkOutage(targetName, description string, clientConfig *rest.Config) {
	ctx := context.TODO()
	endpointCheckClient := v1alpha1.NewForConfigOrDie(clientConfig)
	connectivityChecks, err := endpointCheckClient.PodNetworkConnectivityChecks("openshift-apiserver").List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	failures := []string{}
	flakes := []string{}
	for _, check := range connectivityChecks.Items {
		if !strings.Contains(check.Name, targetName) {
			continue
		}
		for _, serviceOutage := range check.Status.Outages {

			// check to see if all kube-apiserver endpoints experience outages at the same time.
			allFailedAtSameTime := true
			for _, check := range connectivityChecks.Items {
				if !strings.Contains(check.Name, "kubernetes-apiserver-endpoint") {
					continue
				}
				outageAtTheSameTime := false
				for _, endpointOutage := range check.Status.Outages {
					if endpointOutage.Start.Before(&serviceOutage.End) && endpointOutage.End.After(serviceOutage.Start.Time) {
						// the outage overlapped
						outageAtTheSameTime = true
						break
					}
				}
				if !outageAtTheSameTime {
					allFailedAtSameTime = false
				}
			}

			if !allFailedAtSameTime {
				failures = append(failures, fmt.Sprintf("%#v", serviceOutage))
			} else {
				flakes = append(flakes, fmt.Sprintf("%#v", serviceOutage))
			}
		}

	}

	if len(failures) > 0 {
		g.Fail(fmt.Sprintf("for SDN, the %v was inaccessible via the service network IP (compare against `oc -n openshift-apiserver get podnetworkconnectivitychecks` with  kube-apiserver direct endpoint access):\n%v", description, strings.Join(failures, "\n")))
	}
	if len(flakes) > 0 {
		g.Fail(fmt.Sprintf("for kube-apiserver, the %v was inaccessible via the service network IP but the kube-apiserver was down too:\n%v", description, strings.Join(flakes, "\n")))
	}
}

// NetworkOutageUpgradeTest tests that we don't have an outage of the service network from the openshift-apiserver pod
type NetworkOutageUpgradeTest struct {
	daemonSet *appsv1.DaemonSet
}

// Name returns the tracking name of the test.
func (NetworkOutageUpgradeTest) Name() string {
	return "[sig-network][Late] service network access from openshift-apiserver to kube-apiserver from openshift-apiserver to kube-apiserver"
}

// Setup creates a DaemonSet and verifies that it's running
func (t *NetworkOutageUpgradeTest) Setup(f *framework.Framework) {
}

// Test waits until the upgrade has completed and then verifies that the DaemonSet
// is still running
func (t *NetworkOutageUpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	// wait to ensure API is still up after the test ends
	<-done

	confirmNoKubernetesDefaultServiceNetworkOutage(f.ClientConfig())
	confirmNoKubernetesServiceMonitorServiceNetworkOutage(f.ClientConfig())
}

// Teardown cleans up any remaining resources.
func (t *NetworkOutageUpgradeTest) Teardown(f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}
