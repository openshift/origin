package topologymanager

import (
	"fmt"
	"os"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"

	"k8s.io/client-go/dynamic"
	//	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/stretchr/objx"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
)

const (
	FixtureSetupEnvVar    = "OSE_TEST_TOPOLOGY_MANAGER_FIXTURE_SETUP"
	FixtureTeardownEnvVar = "OSE_TEST_TOPOLOGY_MANAGER_FIXTURE_TEARDOWN"

	TopologyManagerLabelKey          = "topology-manager"
	TopologyManagerLabelValueEnabled = "enabled"

	ClusterFeatureGateName = "cluster"
	CPUManagerPolicyStatic = "static"

	KubeletConfigurationName = "enable-topology-manager"
)

type DynClientSet struct {
	dc dynamic.Interface
}

func (dcs DynClientSet) MachineConfigPools() dynamic.ResourceInterface {
	return dcs.dc.Resource(schema.GroupVersionResource{Group: "machineconfiguration.openshift.io", Resource: "machineconfigpools", Version: "v1"})
}

func (dcs DynClientSet) KubeletConfigs() dynamic.ResourceInterface {
	return dcs.dc.Resource(schema.GroupVersionResource{Group: "machineconfiguration.openshift.io", Resource: "kubeletconfigs", Version: "v1"})
}

func NewDynClientSet() (*DynClientSet, error) {
	cfg, err := e2e.LoadConfig()
	if err != nil {
		return nil, err
	}

	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &DynClientSet{
		dc: dc,
	}, nil
}

type TMFixture struct {
	OC     *exutil.CLI
	Client clientset.Interface // shortcut
	MCOCli *DynClientSet

	FeatureGate *configv1.FeatureGate
	MCPName     string
}

func (tmf *TMFixture) Setup(mcpName string) error {
	g.By("Setting up the feature gate")
	fg, err := getFeatureGate(tmf.OC, ClusterFeatureGateName)
	if errors.IsNotFound(err) {
		e2e.Logf("%q feature gate NOT FOUND", ClusterFeatureGateName)
		fg = newEmptyFeatureGate()
	} else if err != nil {
		return err
	}

	feature := string(configv1.LatencySensitive)
	fgEnabled := isFeatureEnabled(fg, feature)
	e2e.Logf("Feature Gate enabled=%v", fgEnabled)
	if !fgEnabled {
		g.By("Enabling the Feature Gate")
		tmf.FeatureGate = fg
		err = tmf.setupFeatureGate(feature)
		if err != nil {
			return err
		}
	}
	g.By("Feature gate setup done")

	g.By("Setting up the Topology Manager")
	// any random node
	node := &workerNodes[0]
	kc, err := getKubeletConfig(tmf.Client, tmf.OC, node)
	if err != nil {
		return err
	}

	tmEnabled := isTopologyManagerEnabled(kc)
	e2e.Logf("TopologyManger enabled=%v", tmEnabled)
	if !tmEnabled {
		g.By("Enabling the Topology Manager")
		tmf.MCPName = mcpName
		err = tmf.setupTopologyManager()
		if err != nil {
			return err
		}
	}

	g.By("Topology Manager setup done")
	return nil
}

func (tmf *TMFixture) setupFeatureGate(feature string) error {
	e2e.Logf("Enabling the feature gate %q", feature)
	// store the current value so we can clean up later

	newFg := tmf.FeatureGate.DeepCopy()
	err := enableFeature(newFg, feature)
	if err != nil {
		return err
	}

	_, err = createOrUpdateFeatureGate(tmf.OC, newFg)
	if err != nil {
		return err
	}

	e2e.Logf("Waiting for the feature gate to be enabled...")
	err = waitForFeatureGate(tmf.OC, 5*time.Minute, feature, true)
	if err != nil {
		return err
	}
	e2e.Logf("Feature gate enabled")

	return nil
}

func newKubeletConfigurationForTopologyManager() objx.Map {
	return objx.New(map[string]interface{}{
		"apiVersion": "machineconfiguration.openshift.io/v1",
		"kind":       "KubeletConfig",
		"metadata": map[string]interface{}{
			"name": KubeletConfigurationName,
		},
		"spec": map[string]interface{}{
			"machineConfigPoolSelector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					TopologyManagerLabelKey: TopologyManagerLabelValueEnabled,
				},
			},
			"kubeletConfig": map[string]interface{}{
				"cpuManagerPolicy":          "static",
				"cpuManagerReconcilePeriod": "10s",
				"topologyManagerPolicy":     "single-numa-node",
			},
		},
	})

}

func generationFromMCP(mcp objx.Map) (int64, error) {
	genValue := mcp.Get("status.observedGeneration")
	if genValue.IsNil() {
		return 0, fmt.Errorf("no observedGeneration found")
	}
	referenceGeneration := genValue.Int64()
	e2e.Logf("current MCP generation: %v", referenceGeneration)
	return referenceGeneration, nil
}

func (tmf *TMFixture) setupTopologyManager() error {
	e2e.Logf("Enabling TopologyManager using MCO in pool %q", tmf.MCPName)

	mcp, err := getMachineConfigPool(tmf.MCOCli, tmf.MCPName)
	if err != nil {
		return err
	}

	referenceGeneration, err := generationFromMCP(mcp)
	if err != nil {
		return err
	}
	e2e.Logf("Setup: reference generation is%v", referenceGeneration)

	_, err = addCustomLabelToMCP(tmf.MCOCli, mcp)
	if err != nil {
		return err
	}
	e2e.Logf("Setup: labels added")

	err = waitForMachineConfigPoolLabels(tmf.MCOCli, 5*time.Minute, tmf.MCPName, true)
	if err != nil {
		return err
	}
	e2e.Logf("Setup: MCP updated")

	kc := newKubeletConfigurationForTopologyManager()
	_, err = createKubeletConfigUsingMCO(tmf.MCOCli, kc)
	if err != nil {
		return err
	}
	e2e.Logf("Setup: Configuring the topology manager")

	e2e.Logf("Waiting for the topology manager to be configured...")
	err = waitForTopologyManager(tmf, 10*time.Minute, tmf.MCPName, referenceGeneration, true)
	if err != nil {
		return err
	}

	e2e.Logf("TopologyManager enabled")
	return nil
}

func (tmf *TMFixture) teardownFeatureGate() error {
	e2e.Logf("Restoring the feature gate state")
	_, err := createOrUpdateFeatureGate(tmf.OC, tmf.FeatureGate)
	if err != nil {
		return err
	}

	e2e.Logf("Waiting for the feature gate to be restored...")
	feature := string(configv1.LatencySensitive)
	err = waitForFeatureGate(tmf.OC, 5*time.Minute, feature, false)
	if err != nil {
		return err
	}
	e2e.Logf("Feature gate restored")

	return nil
}

func (tmf *TMFixture) teardownTopologyManager() error {
	e2e.Logf("Restoring the topology manager state")
	mcp, err := getMachineConfigPool(tmf.MCOCli, tmf.MCPName)
	if err != nil {
		return err
	}

	referenceGeneration, err := generationFromMCP(mcp)
	if err != nil {
		return err
	}
	e2e.Logf("Teardown: reference generation is%v", referenceGeneration)

	_, err = deleteCustomLabelFromMCP(tmf.MCOCli, mcp)
	if err != nil {
		return err
	}
	e2e.Logf("Teardown: labels removed")

	err = waitForMachineConfigPoolLabels(tmf.MCOCli, 5*time.Minute, tmf.MCPName, false)
	if err != nil {
		return err
	}
	e2e.Logf("Teardown: MCP updated")

	err = deleteKubeletConfigUsingMCO(tmf.MCOCli, KubeletConfigurationName)
	if err != nil {
		return err
	}
	e2e.Logf("Teardown: Deconfiguring the topology manager")

	e2e.Logf("Teardown: waiting for the topology manager to be DEconfigured...")
	err = waitForTopologyManager(tmf, 10*time.Minute, tmf.MCPName, referenceGeneration, false)
	if err != nil {
		return err
	}

	e2e.Logf("Teardown: TopologyManager disabled")
	return nil
}

func (tmf *TMFixture) Teardown(mcpName string) error {
	var err error

	if mcpName != "" {
		e2e.Logf("MCP name overridden: %q", mcpName)
		tmf.MCPName = mcpName
	}

	if tmf.MCPName == "" {
		e2e.Logf("No need to restore the topology manager configuration")
	} else {
		err = tmf.teardownTopologyManager()
		if err != nil {
			return err
		}
	}

	if tmf.FeatureGate == nil {
		e2e.Logf("No need to restore the cluster feature gate")
	} else {
		err = tmf.teardownFeatureGate()
		if err != nil {
			return err
		}
	}

	return nil
}

func createKubeletConfigUsingMCO(mcoCli *DynClientSet, kc objx.Map) (objx.Map, error) {
	u := unstructured.Unstructured{Object: kc}
	x, err := mcoCli.KubeletConfigs().Create(&u, metav1.CreateOptions{})
	return unstructuredToObjxMap(x, err)
}

func labelsFromMCP(mcp objx.Map) (map[string]interface{}, error) {
	e2e.Logf("Labels %v", mcp.Get("metadata.labels"))
	// TODO: what if no labels?
	labels := (mcp.Get("metadata.labels")).Data().(map[string]interface{})
	return labels, nil
}

func addCustomLabelToMCP(mcoCli *DynClientSet, mcp objx.Map) (objx.Map, error) {
	labels, _ := labelsFromMCP(mcp)
	labels[TopologyManagerLabelKey] = TopologyManagerLabelValueEnabled
	e2e.Logf("labels being updated: %v", labels)
	return updateMCP(mcoCli, mcp)
}

func deleteCustomLabelFromMCP(mcoCli *DynClientSet, mcp objx.Map) (objx.Map, error) {
	labels, _ := labelsFromMCP(mcp)
	delete(labels, TopologyManagerLabelKey)
	e2e.Logf("labels being updated: %v", labels)
	return updateMCP(mcoCli, mcp)
}

func waitForMachineConfigPoolLabels(mcoCli *DynClientSet, timeout time.Duration, mcpName string, desiredState bool) error {
	err := wait.Poll(5*time.Second, timeout,
		func() (bool, error) {
			mcp, err := getMachineConfigPool(mcoCli, mcpName)
			if err != nil {
				e2e.Logf("Failed getting MCPs: %v", err)
				return false, nil // Ignore this error (nil) and try again in "Poll" time
			}

			labels, _ := labelsFromMCP(mcp)
			enabled := (labels[TopologyManagerLabelKey] == TopologyManagerLabelValueEnabled)
			e2e.Logf("enabled=%v desired=%v", enabled, desiredState)
			return enabled == desiredState, nil
		})
	return err

}

func unstructuredToObjxMap(x *unstructured.Unstructured, err error) (objx.Map, error) {
	if err != nil {
		return nil, err
	}
	return objx.Map(x.Object), nil
}

func updateMCP(mcoCli *DynClientSet, mcp objx.Map) (objx.Map, error) {
	u := unstructured.Unstructured{Object: mcp}
	x, err := mcoCli.MachineConfigPools().Update(&u, metav1.UpdateOptions{})
	e2e.Logf("updated MCP error=%v", err)
	return unstructuredToObjxMap(x, err)
}

func deleteKubeletConfigUsingMCO(mcoCli *DynClientSet, kcName string) error {
	opts := metav1.DeleteOptions{}
	return mcoCli.MachineConfigPools().Delete(kcName, &opts)
}

func getMachineConfigPool(mcoCli *DynClientSet, mcpName string) (objx.Map, error) {
	x, err := mcoCli.MachineConfigPools().Get(mcpName, metav1.GetOptions{})
	return unstructuredToObjxMap(x, err)
}

func isMCPUpdated(mcp objx.Map, referenceGeneration int64) bool {
	genValue := mcp.Get("status.observedGeneration")
	if genValue.IsNil() {
		return false
	}
	currentGeneration := genValue.Int64()
	if currentGeneration <= referenceGeneration {
		e2e.Logf("current generation %v reference generation %v", currentGeneration, referenceGeneration)
		return false
	}

	updatedMachineCountValue := mcp.Get("status.updatedMachineCount")
	machineCountValue := mcp.Get("status.machineCount")
	if updatedMachineCountValue.IsNil() || machineCountValue.IsNil() {
		return false
	}
	updatedMachines := updatedMachineCountValue.Int64()
	totalMachines := machineCountValue.Int64()
	if updatedMachines < totalMachines {
		e2e.Logf("machine updated: %v/%v", updatedMachines, totalMachines)
		return false
	}
	return true
}

func waitForTopologyManager(tmf *TMFixture, timeout time.Duration, mcpName string, referenceGeneration int64, desiredState bool) error {
	err := wait.Poll(5*time.Second, timeout,
		func() (bool, error) {
			mcp, err := getMachineConfigPool(tmf.MCOCli, mcpName)
			if err != nil {
				e2e.Logf("Failed getting MCPs: %v", err)
				return false, nil // Ignore this error (nil) and try again in "Poll" time
			}
			updated := isMCPUpdated(mcp, referenceGeneration)
			if !updated {
				return false, nil
			}
			e2e.Logf("MCP %q updated=%v", mcpName, updated)

			node := &workerNodes[0]
			kc, err := getKubeletConfig(tmf.Client, tmf.OC, node)
			if err != nil {
				e2e.Logf("Failed getting KCs: %v", err)
				return false, nil // Ignore this error (nil) and try again in "Poll" time
			}
			enabled := isTopologyManagerEnabled(kc)
			e2e.Logf("enabled=%v desired=%v", enabled, desiredState)
			return enabled == desiredState, nil
		})
	return err
}

func newEmptyKubeletConfiguration() *kubeletconfigv1beta1.KubeletConfiguration {
	return &kubeletconfigv1beta1.KubeletConfiguration{}
}

func isTopologyManagerEnabled(kc *kubeletconfigv1beta1.KubeletConfiguration) bool {
	return kc.CPUManagerPolicy == CPUManagerPolicyStatic &&
		kc.CPUManagerReconcilePeriod.Milliseconds() > 0 && // intentionally more lax check than what we set
		kc.TopologyManagerPolicy == kubeletconfigv1beta1.SingleNumaNodeTopologyManager
}

func enableTopologyManager(kc *kubeletconfigv1beta1.KubeletConfiguration) error {
	kc.CPUManagerPolicy = CPUManagerPolicyStatic
	kc.CPUManagerReconcilePeriod = metav1.Duration{Duration: 5 * time.Second}
	kc.TopologyManagerPolicy = kubeletconfigv1beta1.BestEffortTopologyManagerPolicy
	return nil
}

func isFeatureEnabled(fg *configv1.FeatureGate, feature string) bool {
	return strings.Contains(string(fg.Spec.FeatureSet), string(configv1.LatencySensitive))
}

func enableFeature(fg *configv1.FeatureGate, feature string) error {
	// TODO append?
	fg.Spec.FeatureSet = configv1.FeatureSet(feature)
	return nil
}

func newEmptyFeatureGate() *configv1.FeatureGate {
	return &configv1.FeatureGate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: configv1.GroupVersion.String(),
			Kind:       "FeatureGate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: ClusterFeatureGateName,
		},
	}
}

func getFeatureGate(oc *exutil.CLI, name string) (*configv1.FeatureGate, error) {
	cli := oc.AdminConfigClient()
	return cli.ConfigV1().FeatureGates().Get(name, metav1.GetOptions{})
}

func createOrUpdateFeatureGate(oc *exutil.CLI, fg *configv1.FeatureGate) (*configv1.FeatureGate, error) {
	cli := oc.AdminConfigClient()
	currentFg, err := getFeatureGate(oc, fg.Name)
	if errors.IsNotFound(err) {
		e2e.Logf("Create feature-gate %q", fg.Name)
		return cli.ConfigV1().FeatureGates().Create(fg)
	}

	if err != nil {
		return nil, err
	}

	e2e.Logf("Update feature-gate %q: %q -> %q", fg.Name, currentFg.Spec.FeatureSet, fg.Spec.FeatureSet)
	newFg := currentFg.DeepCopy()
	newFg.Spec.FeatureSet = fg.Spec.FeatureSet
	return cli.ConfigV1().FeatureGates().Update(newFg)
}

func waitForFeatureGate(oc *exutil.CLI, timeout time.Duration, feature string, desiredState bool) error {
	err := wait.Poll(2*time.Second, timeout,
		func() (bool, error) {
			fg, err := getFeatureGate(oc, ClusterFeatureGateName)
			if err != nil {
				e2e.Logf("Failed getting FGs: %v", err)
				return false, nil // Ignore this error (nil) and try again in "Poll" time
			}

			enabled := isFeatureEnabled(fg, feature)
			e2e.Logf("FeatureSet = %q enabled=%v desiredState=%v", fg.Spec.FeatureSet, enabled, desiredState)
			return (enabled == desiredState), nil
		})
	return err
}

var (
	oc        *exutil.CLI
	tmFixture *TMFixture

	roleWorkerLabel string
	workerNodes     []corev1.Node
)

func Before() {
	var err error
	o.Expect(oc).ToNot(o.BeNil())

	client := oc.KubeFramework().ClientSet
	o.Expect(client).ToNot(o.BeNil())

	roleWorkerLabel = getRoleWorkerLabel()
	workerNodes, err = getNodeByRole(client, roleWorkerLabel)
	e2e.ExpectNoError(err)
	o.Expect(workerNodes).ToNot(o.BeEmpty())

	mcoCli, err := NewDynClientSet()
	o.Expect(err).NotTo(o.HaveOccurred())

	tmFixture = &TMFixture{
		OC:     oc,
		Client: client,
		MCOCli: mcoCli,
	}

	if val := os.Getenv(FixtureSetupEnvVar); strings.ToUpper(val) == "SKIP" {
		return
	}

	err = tmFixture.Setup(getMachineConfigPoolName())
	o.Expect(err).NotTo(o.HaveOccurred())
}

func After() {
	val := os.Getenv(FixtureTeardownEnvVar)
	if strings.ToUpper(val) == "SKIP" {
		return
	}

	mcpName := ""
	if strings.ToUpper(val) == "FORCE" {
		mcpName = getMachineConfigPoolName()
	}
	err := tmFixture.Teardown(mcpName)
	o.Expect(err).NotTo(o.HaveOccurred())
}
