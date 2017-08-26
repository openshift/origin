package network

// Set up test environment needed for network diagnostics
import (
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/diagnostics/networkpod/util"
	diagutil "github.com/openshift/origin/pkg/diagnostics/util"
	"github.com/openshift/origin/pkg/oc/cli/config"
	"github.com/openshift/origin/pkg/sdn"
	sdnapi "github.com/openshift/origin/pkg/sdn/apis/network"
)

func (d *NetworkDiagnostic) TestSetup() error {
	d.nsName1 = names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-", util.NetworkDiagNamespacePrefix))
	d.nsName2 = names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-", util.NetworkDiagNamespacePrefix))

	nsList := []string{d.nsName1, d.nsName2}
	if sdn.IsOpenShiftMultitenantNetworkPlugin(d.pluginName) {
		d.globalnsName1 = names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-", util.NetworkDiagGlobalNamespacePrefix))
		nsList = append(nsList, d.globalnsName1)
		d.globalnsName2 = names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-", util.NetworkDiagGlobalNamespacePrefix))
		nsList = append(nsList, d.globalnsName2)
	}

	for _, name := range nsList {
		// Create a new namespace for network diagnostics
		ns := &kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: map[string]string{"openshift.io/node-selector": ""}}}
		if _, err := d.KubeClient.Core().Namespaces().Create(ns); err != nil {
			return fmt.Errorf("Creating namespace %q failed: %v", name, err)
		}
		if strings.HasPrefix(name, util.NetworkDiagGlobalNamespacePrefix) {
			if err := d.makeNamespaceGlobal(name); err != nil {
				return fmt.Errorf("Making namespace %q global failed: %v", name, err)
			}
		}
	}

	// Store kubeconfig as secret, used by network diagnostic pod
	kconfigData, err := d.getKubeConfig()
	if err != nil {
		return fmt.Errorf("Fetching kube config for network pod failed: %v", err)
	}
	secret := &kapi.Secret{}
	secret.Name = util.NetworkDiagSecretName
	secret.Data = map[string][]byte{strings.ToLower(kclientcmd.RecommendedConfigPathEnvVar): kconfigData}
	if _, err = d.KubeClient.Core().Secrets(d.nsName1).Create(secret); err != nil {
		return fmt.Errorf("Creating secret %q failed: %v", secret.Name, err)
	}

	// Create test pods and services on all valid nodes
	if err := d.createTestPodAndService(nsList); err != nil {
		// Failed to create test pods/services on some nodes
		d.res.Error("DNet3001", err, fmt.Sprintf("Failed to create network diags test pod and service: %v", err))
	}
	// Wait for test pods and services to be up and running on all valid nodes
	if err = d.waitForTestPodAndService(nsList); err != nil {
		return fmt.Errorf("Failed to run network diags test pod and service: %v", err)
	}
	return nil
}

func (d *NetworkDiagnostic) Cleanup() {
	// Deleting namespaces will delete corresponding service accounts/pods in the namespace automatically.
	d.KubeClient.Core().Namespaces().Delete(d.nsName1, nil)
	d.KubeClient.Core().Namespaces().Delete(d.nsName2, nil)
	d.KubeClient.Core().Namespaces().Delete(d.globalnsName1, nil)
	d.KubeClient.Core().Namespaces().Delete(d.globalnsName2, nil)
}

func (d *NetworkDiagnostic) getPodList(nsName, prefix string) (*kapi.PodList, error) {
	podList, err := d.KubeClient.Core().Pods(nsName).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	filteredPodList := &kapi.PodList{}
	for _, pod := range podList.Items {
		if strings.HasPrefix(pod.Name, prefix) {
			filteredPodList.Items = append(filteredPodList.Items, pod)
		}
	}
	return filteredPodList, nil
}

func (d *NetworkDiagnostic) waitForNetworkPod(nsName, prefix string, backoff wait.Backoff, validPhases []kapi.PodPhase) error {
	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		podList, err := d.getPodList(nsName, prefix)
		if err != nil {
			return false, err
		}

		for _, pod := range podList.Items {
			foundValidPhase := false
			for _, phase := range validPhases {
				if pod.Status.Phase == phase {
					foundValidPhase = true
					break
				}
			}
			if !foundValidPhase {
				return false, nil
			}
		}
		return true, nil
	})
}

func (d *NetworkDiagnostic) createTestPodAndService(nsList []string) error {
	errList := []error{}
	for _, node := range d.nodes {
		for _, nsName := range nsList {
			// Create 2 pods and a service in global and non-global network diagnostic namespaces
			var testPodName string
			for i := 0; i < 2; i++ {
				testPodName = names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-", util.NetworkDiagTestPodNamePrefix))
				// Create network diags test pod on the given node for the given namespace
				pod := GetTestPod(d.TestPodImage, d.TestPodProtocol, testPodName, node.Name, d.TestPodPort)
				if _, err := d.KubeClient.Core().Pods(nsName).Create(pod); err != nil {
					errList = append(errList, fmt.Errorf("Creating network diagnostic test pod '%s/%s' on node %q failed: %v", nsName, testPodName, node.Name, err))
					continue
				}
			}

			// Create network diags test service on the given node for the given namespace
			testServiceName := names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-", util.NetworkDiagTestServiceNamePrefix))
			service := GetTestService(testServiceName, testPodName, d.TestPodProtocol, node.Name, d.TestPodPort)
			if _, err := d.KubeClient.Core().Services(nsName).Create(service); err != nil {
				errList = append(errList, fmt.Errorf("Creating network diagnostic test service '%s/%s' on node %q failed: %v", nsName, testServiceName, node.Name, err))
				continue
			}
		}
	}
	return kerrors.NewAggregate(errList)
}

func (d *NetworkDiagnostic) waitForTestPodAndService(nsList []string) error {
	errList := []error{}
	validPhases := []kapi.PodPhase{kapi.PodRunning, kapi.PodSucceeded, kapi.PodFailed}
	for _, name := range nsList {
		backoff := wait.Backoff{Steps: 36, Duration: time.Second, Factor: 1.1} // timeout: ~5 mins
		if err := d.waitForNetworkPod(name, util.NetworkDiagTestPodNamePrefix, backoff, validPhases); err != nil {
			errList = append(errList, err)
		}
	}

	if totalPods, runningPods, err := d.getCountOfTestPods(nsList); err == nil {
		// Perform network diagnostic checks if we are able to launch decent number of test pods (at least 50%)
		if runningPods != totalPods {
			if runningPods >= (totalPods / 2) {
				d.res.Warn("DNet3002", nil, fmt.Sprintf("Failed to run some network diags test pods: %d, So some network diagnostic checks may be skipped.", (totalPods-runningPods)))
				return nil
			} else {
				errList = append(errList, fmt.Errorf("Failed to run network diags test pods, failed: %d, total: %d", (totalPods-runningPods), totalPods))
			}
		}
	}
	return kerrors.NewAggregate(errList)
}

func (d *NetworkDiagnostic) getCountOfTestPods(nsList []string) (int, int, error) {
	totalPodCount := 0
	runningPodCount := 0
	for _, name := range nsList {
		podList, err := d.getPodList(name, util.NetworkDiagTestPodNamePrefix)
		if err != nil {
			return -1, -1, err
		}
		totalPodCount += len(podList.Items)

		for _, pod := range podList.Items {
			if pod.Status.Phase == kapi.PodRunning {
				runningPodCount += 1
			}
		}
	}
	return totalPodCount, runningPodCount, nil
}

func (d *NetworkDiagnostic) makeNamespaceGlobal(nsName string) error {
	backoff := wait.Backoff{
		Steps:    30,
		Duration: 500 * time.Millisecond,
		Factor:   1.1,
	}
	var netns *sdnapi.NetNamespace
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		var err error
		netns, err = d.OSClient.NetNamespaces().Get(nsName, metav1.GetOptions{})
		if kerrs.IsNotFound(err) {
			// NetNamespace not created yet
			return false, nil
		} else if err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return err
	}

	sdn.SetChangePodNetworkAnnotation(netns, sdn.GlobalPodNetwork, "")

	if _, err = d.OSClient.NetNamespaces().Update(netns); err != nil {
		return err
	}

	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		updatedNetNs, err := d.OSClient.NetNamespaces().Get(netns.NetName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if _, _, err = sdn.GetChangePodNetworkAnnotation(updatedNetNs); err == sdn.ErrorPodNetworkAnnotationNotFound {
			return true, nil
		}
		// Pod network change not applied yet
		return false, nil
	})
}

func (d *NetworkDiagnostic) getKubeConfig() ([]byte, error) {
	// KubeConfig path search order:
	// 1. User given config path
	// 2. Default admin config paths
	// 3. Default openshift client config search paths
	paths := []string{}
	paths = append(paths, d.ClientFlags.Lookup(config.OpenShiftConfigFlagName).Value.String())
	paths = append(paths, diagutil.AdminKubeConfigPaths...)
	paths = append(paths, config.NewOpenShiftClientConfigLoadingRules().Precedence...)

	for _, path := range paths {
		if configData, err := ioutil.ReadFile(path); err == nil {
			return configData, nil
		}
	}
	return nil, fmt.Errorf("Unable to find kube config")
}
