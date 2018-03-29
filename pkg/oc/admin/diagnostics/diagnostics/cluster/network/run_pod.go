package network

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/util/sets"
	kvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storage/names"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	networktypedclient "github.com/openshift/origin/pkg/network/generated/internalclientset/typed/network/internalversion"
	networkpoddiag "github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/cluster/network/in_pod"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/cluster/network/in_pod/util"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/log"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
	osclientcmd "github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const (
	NetworkDiagnosticName          = "NetworkCheck"
	FlagNetworkDiagLogDir          = "logdir"
	FlagNetworkDiagPodImage        = "pod-image"
	FlagNetworkDiagTestPodImage    = "test-pod-image"
	FlagNetworkDiagTestPodProtocol = "test-pod-protocol"
	FlagNetworkDiagTestPodPort     = "test-pod-port"
)

// NetworkDiagnostic is a diagnostic that runs a network diagnostic pod and relays the results.
type NetworkDiagnostic struct {
	KubeClient           kclientset.Interface
	NetNamespacesClient  networktypedclient.NetNamespacesGetter
	ClusterNetworkClient networktypedclient.ClusterNetworksGetter
	ClientFlags          *flag.FlagSet
	Level                int
	Factory              *osclientcmd.Factory
	RawConfig            *clientcmdapi.Config
	PreventModification  bool
	LogDir               string
	PodImage             string
	TestPodImage         string
	TestPodProtocol      string
	TestPodPort          int

	pluginName    string
	nodes         []kapi.Node
	nsName1       string
	nsName2       string
	globalnsName1 string
	globalnsName2 string
	res           types.DiagnosticResult
}

// Name is part of the Diagnostic interface and just returns name.
func (d *NetworkDiagnostic) Name() string {
	return NetworkDiagnosticName
}

// Description is part of the Diagnostic interface and provides a user-focused description of what the diagnostic does.
func (d *NetworkDiagnostic) Description() string {
	return "Create a pod on all schedulable nodes and run network diagnostics from the application standpoint"
}

func (d *NetworkDiagnostic) Requirements() (client bool, host bool) {
	return true, false
}

func (d *NetworkDiagnostic) AvailableParameters() []types.Parameter {
	return []types.Parameter{
		{FlagNetworkDiagLogDir, "Path to store diagnostic results in case of errors", &d.LogDir, util.NetworkDiagDefaultLogDir},
		{FlagNetworkDiagPodImage, "Image to use for diagnostic pod", &d.PodImage, util.GetNetworkDiagDefaultPodImage()},
		{FlagNetworkDiagTestPodImage, "Image to use for diagnostic test pod", &d.TestPodImage, util.GetNetworkDiagDefaultTestPodImage()},
		{FlagNetworkDiagTestPodProtocol, "Protocol used to connect to diagnostic test pod", &d.TestPodProtocol, util.NetworkDiagDefaultTestPodProtocol},
		{FlagNetworkDiagTestPodPort, "Serving port on the diagnostic test pod", &d.TestPodPort, util.NetworkDiagDefaultTestPodPort},
	}
}

func (d *NetworkDiagnostic) Complete(logger *log.Logger) error {
	logdir, err := filepath.Abs(d.LogDir)
	if err != nil {
		return err
	}
	if path, err := os.Stat(d.LogDir); err == nil && !path.Mode().IsDir() {
		return fmt.Errorf("Network log path %q exists but is not a directory", d.LogDir)
	}
	d.LogDir = logdir

	supportedProtocols := sets.NewString(string(kapi.ProtocolTCP), string(kapi.ProtocolUDP))
	if !supportedProtocols.Has(d.TestPodProtocol) {
		return fmt.Errorf("invalid protocol for network diagnostic test pod. Supported protocols: %s", strings.Join(supportedProtocols.List(), ","))
	}
	if kvalidation.IsValidPortNum(d.TestPodPort) != nil {
		return fmt.Errorf("invalid port for network diagnostic test pod. Must be in the range 1 to 65535.")
	}
	return nil
}

// CanRun is part of the Diagnostic interface; it determines if the conditions are right to run this diagnostic.
func (d *NetworkDiagnostic) CanRun() (bool, error) {
	if d.PreventModification {
		return false, errors.New("running the network diagnostic pod is an API change, which is prevented as you indicated")
	} else if d.KubeClient == nil {
		return false, errors.New("must have kube client")
	} else if d.NetNamespacesClient == nil || d.ClusterNetworkClient == nil {
		return false, errors.New("must have openshift client")
	}
	return true, nil
}

// Check is part of the Diagnostic interface; it runs the actual diagnostic logic
func (d *NetworkDiagnostic) Check() types.DiagnosticResult {
	d.res = types.NewDiagnosticResult(NetworkDiagnosticName)

	var err error
	var ok bool
	d.pluginName, ok, err = util.GetOpenShiftNetworkPlugin(d.ClusterNetworkClient)
	if err != nil {
		d.res.Error("DNet2001", err, fmt.Sprintf("Checking network plugin failed. Error: %s", err))
		return d.res
	}
	if !ok {
		d.res.Info("DNet2002", "Skipping network diagnostics check. Reason: Not using openshift network plugin.")
		return d.res
	}

	d.nodes, err = util.GetSchedulableNodes(d.KubeClient)
	if err != nil {
		d.res.Error("DNet2003", err, fmt.Sprintf("Fetching schedulable nodes failed. Error: %s", err))
		return d.res
	}
	if len(d.nodes) == 0 {
		d.res.Warn("DNet2004", nil, "Skipping network checks. Reason: No schedulable/ready nodes found.")
		return d.res
	}

	// Abort and clean up if there is an interrupt/terminate signal while running network diagnostics
	done := make(chan bool, 1)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		d.res.Warn("DNet2014", nil, "Interrupt received; aborting network diagnostic.")
		done <- true
	}()
	go func() {
		d.runNetworkDiagnostic()
		done <- true
	}()
	<-done
	signal.Stop(sig)
	d.Cleanup()

	return d.res
}

func (d *NetworkDiagnostic) runNetworkDiagnostic() {
	// Setup test environment
	if err := d.TestSetup(); err != nil {
		d.res.Error("DNet2005", err, fmt.Sprintf("Setting up test environment for network diagnostics failed: %v", err))
		return
	}

	// Need to show summary at least
	loglevel := d.Level
	if loglevel > 2 {
		loglevel = 2
	}

	// Network diagnostics has 2 phases: Testing and Collection phase.
	// In Testing phase, various network related diagnostics are performed on every node and results are stored on the respective nodes.
	// In Collection phase, results from each node are moved to the user machine where the CLI cmd is executed.

	// TEST Phase: Run network diagnostic pod on all valid nodes in parallel
	command := fmt.Sprintf("oc adm diagnostics %s -l %d", networkpoddiag.InPodNetworkCheckRecommendedName, loglevel)
	if err := d.runNetworkPod(command); err != nil {
		d.res.Error("DNet2006", err, err.Error())
		return
	}
	// Wait for network diagnostic pod completion (timeout: ~3 mins)
	backoff := wait.Backoff{Steps: 39, Duration: 500 * time.Millisecond, Factor: 1.1}
	if err := d.waitForNetworkPod(d.nsName1, util.NetworkDiagPodNamePrefix, backoff, []kapi.PodPhase{kapi.PodSucceeded, kapi.PodFailed}); err != nil {
		d.res.Error("DNet2007", err, err.Error())
		return
	}
	// Gather logs from network diagnostic pod on all valid nodes
	diagsFailed := false
	if err := d.CollectNetworkPodLogs(); err != nil {
		d.res.Error("DNet2008", err, err.Error())
		diagsFailed = true
	}

	// Collection Phase: Run network diagnostic pod on all valid nodes
	// Block the network diagnostic pod on every node so that results can be moved to the user machine.
	command = "sleep 1000"
	if err := d.runNetworkPod(command); err != nil {
		d.res.Error("DNet2009", err, err.Error())
		return
	}

	// Wait for network diagnostic pod to start (timeout: ~5 mins)
	backoff = wait.Backoff{Steps: 37, Duration: time.Second, Factor: 1.1}
	if err := d.waitForNetworkPod(d.nsName1, util.NetworkDiagPodNamePrefix, backoff, []kapi.PodPhase{kapi.PodRunning, kapi.PodFailed, kapi.PodSucceeded}); err != nil {
		d.res.Error("DNet2010", err, err.Error())
		// Do not bail out here, collect what ever info is available from all valid nodes
	}

	if err := d.CollectNetworkInfo(diagsFailed); err != nil {
		d.res.Error("DNet2011", err, err.Error())
	}

	if diagsFailed {
		d.res.Info("DNet2012", fmt.Sprintf("Additional info collected under %q for further analysis", d.LogDir))
	}
	return
}

func (d *NetworkDiagnostic) runNetworkPod(command string) error {
	for _, node := range d.nodes {
		podName := names.SimpleNameGenerator.GenerateName(fmt.Sprintf("%s-", util.NetworkDiagPodNamePrefix))

		pod := GetNetworkDiagnosticsPod(d.PodImage, command, podName, node.Name)
		_, err := d.KubeClient.Core().Pods(d.nsName1).Create(pod)
		if err != nil {
			return fmt.Errorf("Creating network diagnostic pod %q on node %q with command %q failed: %v", podName, node.Name, command, err)
		}
		d.res.Debug("DNet2013", fmt.Sprintf("Created network diagnostic pod %q on node %q with command: %q", podName, node.Name, command))
	}
	return nil
}
