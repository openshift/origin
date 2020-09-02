package networking

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/openshift/origin/test/extended/util/disruption"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"github.com/openshift/client-go/operatorcontrolplane/clientset/versioned/typed/operatorcontrolplane/v1alpha1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"
)

var _ = g.Describe("[sig-network][Late] service network access from openshift-apiserver to kube-apiserver", func() {
	f := framework.NewDefaultFramework("service-network-01")

	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithoutNamespace("service-network-openshift-apiserver")
	)

	g.It("shouldn't report outage to kubernetes-service", func() {
		confirmNoKubernetesDefaultServiceNetworkOutage(f, oc.AdminConfig())
	})

	g.It("shouldn't report outage to kubernetes-apiserver-service", func() {
		confirmNoKubernetesServiceMonitorServiceNetworkOutage(f, oc.AdminConfig())
	})
})

var _ = g.Describe("[sig-kube-apiserver][Late] load balancer access from kube-apiserver to kube-apiserver", func() {
	f := framework.NewDefaultFramework("lb-01")

	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithoutNamespace("loadbalancer-kube-apiserver")
	)

	g.It("shouldn't report outage to external load balancer", func() {
		confirmNoHostNetworkExternalLoadBalancerOutage(f, oc.AdminConfig())
	})

	g.It("shouldn't report outage to internal load balancer", func() {
		confirmNoHostNetworkInternalLoadBalancerOutage(f, oc.AdminConfig())
	})
})

var _ = g.Describe("[sig-kube-apiserver][Late] load balancer access from openshift-apiserver to kube-apiserver", func() {
	f := framework.NewDefaultFramework("lb-02")

	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithoutNamespace("loadbalancer-kube-apiserver")
	)

	g.It("shouldn't report outage to external load balancer", func() {
		confirmNoPodNetworkExternalLoadBalancerOutage(f, oc.AdminConfig())
	})

	g.It("shouldn't report outage to internal load balancer", func() {
		confirmNoPodNetworkInternalLoadBalancerOutage(f, oc.AdminConfig())
	})
})

// testRunCount is a map of test full text to the number of times it ran.  We use this to ensure we don't run tests
// that must flake and not fail.  We make tests like this so that we can detect conditions without causing insta-fails
// across the stack
var (
	testRunCountLock = sync.Mutex{}
	testRunCount     = map[string]int{}
)

func shouldForceTestSuccess() bool {
	testRunCountLock.Lock()
	defer testRunCountLock.Unlock()

	test := g.CurrentGinkgoTestDescription().FullTestText
	curr := testRunCount[test]
	framework.Logf("testRunCount[%q]==%d", test, curr)
	testRunCount[test] = curr + 1
	// if we have run before, then we should run again
	if curr > 0 {
		return true
	}
	return false
}

func confirmNoHostNetworkExternalLoadBalancerOutage(f *framework.Framework, clientConfig *rest.Config) {
	confirmNoLoadBalancerOutage(f, "openshift-kube-apiserver", "load-balancer-api-external", "api.route from host network", clientConfig)
}

func confirmNoHostNetworkInternalLoadBalancerOutage(f *framework.Framework, clientConfig *rest.Config) {
	confirmNoLoadBalancerOutage(f, "openshift-kube-apiserver", "load-balancer-api-internal", "api-int.route from host network", clientConfig)
}

func confirmNoPodNetworkExternalLoadBalancerOutage(f *framework.Framework, clientConfig *rest.Config) {
	confirmNoLoadBalancerOutage(f, "openshift-apiserver", "load-balancer-api-external", "api.route from pod network", clientConfig)
}

func confirmNoPodNetworkInternalLoadBalancerOutage(f *framework.Framework, clientConfig *rest.Config) {
	confirmNoLoadBalancerOutage(f, "openshift-apiserver", "load-balancer-api-internal", "api-int.route from pod network", clientConfig)
}

func confirmNoLoadBalancerOutage(f *framework.Framework, namespace, targetName, description string, clientConfig *rest.Config) {
	if shouldForceTestSuccess() {
		return
	}

	ctx := context.TODO()
	endpointCheckClient := v1alpha1.NewForConfigOrDie(clientConfig)
	connectivityChecks, err := endpointCheckClient.PodNetworkConnectivityChecks(namespace).List(ctx, metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	failures := []string{}
	for _, check := range connectivityChecks.Items {
		if !strings.Contains(check.Name, targetName) {
			continue
		}
		for _, serviceOutage := range check.Status.Outages {
			failures = append(failures, fmt.Sprintf("%#v", serviceOutage))
		}

	}

	if len(failures) > 0 {
		failString := fmt.Sprintf("%v was inaccessible:\n%v", description, strings.Join(failures, "\n"))
		disruption.RecordJUnitResult(f, g.CurrentGinkgoTestDescription().FullTestText+"---fake", 0, failString)
		disruption.RecordJUnitResult(f, g.CurrentGinkgoTestDescription().FullTestText+"---fake", 0, "") // so we flake
		// g.Fail(failString)
	}
}

func confirmNoKubernetesDefaultServiceNetworkOutage(f *framework.Framework, clientConfig *rest.Config) {
	confirmNoServiceNetworkOutage(f, "kubernetes-default-service", "KUBERNETES_SERVICE_HOST:KUBERNETES_SERVICE_PORT", clientConfig)
}

func confirmNoKubernetesServiceMonitorServiceNetworkOutage(f *framework.Framework, clientConfig *rest.Config) {
	confirmNoServiceNetworkOutage(f, "kubernetes-apiserver-service", "`oc -n openshift-kube-apiserver get services/apiserver`", clientConfig)
}

func confirmNoServiceNetworkOutage(f *framework.Framework, targetName, description string, clientConfig *rest.Config) {
	if shouldForceTestSuccess() {
		return
	}

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
		failString := fmt.Sprintf("for SDN, the %v was inaccessible via the service network IP (compare against `oc -n openshift-apiserver get podnetworkconnectivitychecks` with  kube-apiserver direct endpoint access):\n%v", description, strings.Join(failures, "\n"))
		disruption.RecordJUnitResult(f, g.CurrentGinkgoTestDescription().FullTestText+"---fake", 0, failString)
		disruption.RecordJUnitResult(f, g.CurrentGinkgoTestDescription().FullTestText+"---fake", 0, "") // so we flake
		//g.Fail(failString)
		return
	}
	if len(flakes) > 0 {
		failString := fmt.Sprintf("for kube-apiserver, the %v was inaccessible via the service network IP but the kube-apiserver was down too:\n%v", description, strings.Join(flakes, "\n"))
		disruption.RecordJUnitResult(f, g.CurrentGinkgoTestDescription().FullTestText+"---fake", 0, failString)
		disruption.RecordJUnitResult(f, g.CurrentGinkgoTestDescription().FullTestText+"---fake", 0, "") // so we flake
		//g.Fail(failString)
		return
	}
}

// NetworkOutageUpgradeTest tests that we don't have an outage of the service network from the openshift-apiserver pod
type NetworkOutageUpgradeTest struct {
}

// Name returns the tracking name of the test.
func (NetworkOutageUpgradeTest) Name() string {
	return "[sig-network][Late] service network access from openshift-apiserver to kube-apiserver from openshift-apiserver to kube-apiserver"
}

// Setup creates a DaemonSet and verifies that it's running
func (t *NetworkOutageUpgradeTest) Setup(f *framework.Framework) {
}

func (t *NetworkOutageUpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	// wait to ensure API is still up after the test ends
	<-done

	confirmNoKubernetesDefaultServiceNetworkOutage(f, f.ClientConfig())
	confirmNoKubernetesServiceMonitorServiceNetworkOutage(f, f.ClientConfig())

}

// Teardown cleans up any remaining resources.
func (t *NetworkOutageUpgradeTest) Teardown(f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}

type HostNetworkLoadBalancerOutageUpgradeTest struct {
}

// Name returns the tracking name of the test.
func (HostNetworkLoadBalancerOutageUpgradeTest) Name() string {
	return "[sig-network][Late] load balancer access from host-network to kube-apiserver"
}

// Setup creates a DaemonSet and verifies that it's running
func (t *HostNetworkLoadBalancerOutageUpgradeTest) Setup(f *framework.Framework) {
}

func (t *HostNetworkLoadBalancerOutageUpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	// wait to ensure API is still up after the test ends
	<-done

	confirmNoHostNetworkExternalLoadBalancerOutage(f, f.ClientConfig())
	confirmNoHostNetworkInternalLoadBalancerOutage(f, f.ClientConfig())
}

// Teardown cleans up any remaining resources.
func (t *HostNetworkLoadBalancerOutageUpgradeTest) Teardown(f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}

type PodNetworkLoadBalancerOutageUpgradeTest struct {
}

// Name returns the tracking name of the test.
func (PodNetworkLoadBalancerOutageUpgradeTest) Name() string {
	return "[sig-network][Late] load balancer access from pod-network to kube-apiserver"
}

// Setup creates a DaemonSet and verifies that it's running
func (t *PodNetworkLoadBalancerOutageUpgradeTest) Setup(f *framework.Framework) {
}

func (t *PodNetworkLoadBalancerOutageUpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	// wait to ensure API is still up after the test ends
	<-done

	confirmNoPodNetworkExternalLoadBalancerOutage(f, f.ClientConfig())
	confirmNoPodNetworkInternalLoadBalancerOutage(f, f.ClientConfig())
}

// Teardown cleans up any remaining resources.
func (t *PodNetworkLoadBalancerOutageUpgradeTest) Teardown(f *framework.Framework) {
	// rely on the namespace deletion to clean up everything
}
