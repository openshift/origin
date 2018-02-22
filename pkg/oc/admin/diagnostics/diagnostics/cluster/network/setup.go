package network

// Set up test environment needed for network diagnostics
import (
	"bytes"
	"fmt"
	"strings"
	"time"

	kerrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api/latest"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/printers"

	"github.com/openshift/origin/pkg/network"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/cluster/network/in_pod/util"
)

func (d *NetworkDiagnostic) TestSetup() error {
	d.nsName1 = names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-", util.NetworkDiagNamespacePrefix))
	d.nsName2 = names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-", util.NetworkDiagNamespacePrefix))

	nsList := []string{d.nsName1, d.nsName2}
	if network.IsOpenShiftMultitenantNetworkPlugin(d.pluginName) {
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
		logData, er := d.getPodLogs(nsList)
		if er != nil {
			return fmt.Errorf("Failed to run network diags test pod and service: %v, fetching logs failed: %v", err, er)
		} else {
			return fmt.Errorf("Failed to run network diags test pod and service: %v, details: %s", err, logData)
		}
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
		backoff := wait.Backoff{Steps: 37, Duration: time.Second, Factor: 1.1} // timeout: ~5 mins
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
	} else {
		errList = append(errList, fmt.Errorf("Failed to count test pods: %v ", err))
	}
	return kerrors.NewAggregate(errList)
}

func (d *NetworkDiagnostic) getPodLogs(nsList []string) (string, error) {
	logData := sets.String{}
	errList := []error{}
	limit := int64(1024)

	for _, name := range nsList {
		podList, err := d.getPodList(name, util.NetworkDiagTestPodNamePrefix)
		if err != nil {
			return "", err
		}

		for _, pod := range podList.Items {
			opts := &kapi.PodLogOptions{
				TypeMeta:   pod.TypeMeta,
				Container:  pod.Name,
				Follow:     true,
				LimitBytes: &limit,
			}

			req, err := d.Factory.LogsForObject(&pod, opts, 10*time.Second)
			if err != nil {
				errList = append(errList, err)
				continue
			}
			data, err := req.DoRaw()
			if err != nil {
				errList = append(errList, err)
				continue
			}
			logData.Insert(string(data[:]))
		}
	}
	return strings.Join(logData.List(), ", "), kerrors.NewAggregate(errList)
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
	var netns *networkapi.NetNamespace
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		var err error
		netns, err = d.NetNamespacesClient.NetNamespaces().Get(nsName, metav1.GetOptions{})
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

	network.SetChangePodNetworkAnnotation(netns, network.GlobalPodNetwork, "")

	if _, err = d.NetNamespacesClient.NetNamespaces().Update(netns); err != nil {
		return err
	}

	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		updatedNetNs, err := d.NetNamespacesClient.NetNamespaces().Get(netns.NetName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if _, _, err = network.GetChangePodNetworkAnnotation(updatedNetNs); err == network.ErrorPodNetworkAnnotationNotFound {
			return true, nil
		}
		// Pod network change not applied yet
		return false, nil
	})
}

// turn a raw config object into its string representation (kubeconfig)
func (d *NetworkDiagnostic) getKubeConfig() ([]byte, error) {
	var b bytes.Buffer

	// there does not seem to be a simple DefaultPrinter to invoke; create one
	options := &printers.PrintOptions{
		OutputFormatType: "yaml",
		AllowMissingKeys: true,
	}
	printer, err := cmdutil.PrinterForOptions(meta.NewDefaultRESTMapper(nil, nil), latest.Scheme, nil, []runtime.Decoder{latest.Codec}, options)
	if err != nil {
		return nil, fmt.Errorf("from PrinterForOptions: %#v", err)
	}
	printer = printers.NewVersionedPrinter(printer, latest.Scheme, latest.ExternalVersion)

	if err := printer.PrintObj(d.RawConfig, &b); err != nil {
		return nil, fmt.Errorf("from PrintObj: %#v", err)
	}
	return b.Bytes(), nil
}
