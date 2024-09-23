package kubelet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	"strings"
	"time"

	"github.com/coreos/go-systemd/unit"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
	"github.com/openshift/machine-config-operator/test/helpers"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	podframework "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
)

type SimpleSystemdUnit struct {
	Contents string
	Name     string
}

const workerMCPName = "worker"

var _ = g.Describe("[sig-node][Feature:UserNamespacesSupport]", func() {
	var (
		oc = exutil.NewCLIWithPodSecurityLevel("user-namespaces", admissionapi.LevelBaseline)
	)
	g.Context("[apigroup:config.openshift.io]", func() {
		g.BeforeEach(func() {
			exutil.PreTestDump()
			//TODO remove when user namespaces go GA
			if !exutil.IsTechPreviewNoUpgrade(oc) {
				g.Skip("the test is not expected to work within Tech Preview disabled clusters")
			}
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})
		// This test verifies the kubelet does not create a user namespace pod if it doesn't support them.
		// There are two cases that are being protected against here:
		// - If the kubelet doesn't support user namespaces because of version skew
		// - If the kubelet doesn't support user namespaces because of a FeatureGate rollout/upgrade
		// In either of these cases, the root cause that needs to be verified is the kubelet doesn't have the feature gate
		// on, but the KAS does.
		// Since we can't easily test this through the openshift API, we (hackily) use a MachineConfig to create a drop-in
		// kubelet service configuration. This configuration will add the `--feature-gate="UserNamespacesSupport=false"` flag
		// to the kubelet's CLI, thus disabling the feature gate.
		// Then, we check that the kubelet fails to create a pod that requests a user namespace.
		// A lot of the code is adapted from MCO e2e tests, as well as an e2e-node test case that verifies the same.
		g.It("Kubelet should fail to create a user namespaced pod if featuregate is off [Serial]", func() {
			// instead of a bunch of individual defers, we can run through all of them in a single one
			const mcpName = "userns"
			cleanupFuncs := make([]func(), 0)
			defer func() {
				for _, f := range cleanupFuncs {
					f()
				}
			}()
			originalMCName := getMCName(oc, workerMCPName)
			g.By(fmt.Sprintf("Starting with MC name %s", originalMCName))

			// create mcp
			cleanupFuncs = append(cleanupFuncs, createMCP(oc, mcpName))

			// create mc and wait for it to roll out
			mcName, deleteMCFunc := createKubeletDropinMC(oc, originalMCName, mcpName)
			cleanupFuncs = append(cleanupFuncs, deleteMCFunc)

			// label random node in worker pool
			cleanupFuncs = append(cleanupFuncs, labelRandomNodeFromPool(oc, mcpName, workerMCPName, mcName, originalMCName))

			// create pod
			pod := podframework.MakePod(oc.Namespace(), map[string]string{mcpNameToRole(mcpName): ""}, nil, admissionapi.LevelBaseline, "")
			falseVar := false
			pod.Spec.HostUsers = &falseVar
			g.By("Creating user namespaced pod")
			pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(context.Background(), pod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			// Pod should stay in pending
			// Events would be a better way to tell this, as we could actually read the event from the kubelet saying the exact issue,
			// but events are best-effort and can't be relied on.
			g.By("Consistently checking whether user namespaced pod stays in pending")
			o.Consistently(context.TODO(), func() error {
				p, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.TODO(), pod.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if p.Status.Phase != corev1.PodPending {
					return fmt.Errorf("Pod phase isn't pending")
				}
				return nil
			}, 60*time.Second, 5*time.Second).ShouldNot(o.HaveOccurred())
		})
	})
})

// createKubeletDropinMC takes the existing kubelet.service file, strips it down to just the ExecStart
// stanza, mutates that file to disable user namespaces, and creates a machine config in pool mcpName.
// It returns a function to cleanup all it does
func createKubeletDropinMC(oc *exutil.CLI, originalMCName, mcpName string) (string, func()) {
	g.By("Getting workers MachineConfig")
	workerKubelets, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("machineconfig/01-worker-kubelet", "-o=jsonpath={.spec.config.systemd.units}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Finding kubelet.service among systemd units")
	var systemdUnits []SimpleSystemdUnit
	err = json.Unmarshal([]byte(workerKubelets), &systemdUnits)
	o.Expect(err).NotTo(o.HaveOccurred())
	var kubeletService *SimpleSystemdUnit
	for i := range systemdUnits {
		if systemdUnits[i].Name == "kubelet.service" {
			kubeletService = &systemdUnits[i]
		}
	}
	o.Expect(kubeletService).NotTo(o.BeNil())

	mcadd := createMC("99-userns.conf", mcpName, "/etc/systemd/system/kubelet.service.d/99-userns.conf", usernsDisabledDropin(kubeletService))

	g.By("Creating MC of adapted kubelet service file")
	_, err = oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigs().Create(context.TODO(), mcadd, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return mcadd.Name, func() {
		g.By("Deleting MC of adapted kubelet service file")
		o.Expect(oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigs().Delete(context.TODO(), mcadd.Name, metav1.DeleteOptions{}))
	}
}

// usernsDisabledDropin takes the ExecStart stanza from the existing kubelet.service file,
// prepends "ExecStart=" (to unset the exec start), and then appends the feature-gate flag
// to disable UserNamespacesSupport.
// It is hacky and shouldn't be done in production, this is only done to spoof a situation where
// the kubelet doesn't support user namespaces, but KAS does.
func usernsDisabledDropin(kubeletService *SimpleSystemdUnit) string {
	g.By("Adapting kubelet service file to disable UserNamespacesSupport feature")
	options, err := unit.Deserialize(strings.NewReader(kubeletService.Contents))
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(len(options)).NotTo(o.BeZero())

	execStart := ""
	for _, opt := range options {
		if opt.Section != "Service" {
			continue
		}
		if opt.Name != "ExecStart" {
			continue
		}
		o.Expect(execStart).To(o.BeEmpty())

		execStart = opt.Value
	}
	o.Expect(execStart).NotTo(o.BeEmpty())

	tmpl, err := template.New("dropin").Parse(`[Service]
ExecStart=
ExecStart={{.}} \
--feature-gates="UserNamespacesSupport=false"`)
	o.Expect(err).NotTo(o.HaveOccurred())

	var buf bytes.Buffer
	tmpl.Execute(&buf, execStart)

	return buf.String()
}

// createMC creates a MachineConfig object.
// Adapted from https://github.com/openshift/machine-config-operator/blob/1ac641c0d03/test/e2e/mcd_test.go#L954
func createMC(name, role, filename, data string) *mcfgv1.MachineConfig {
	g.By("Creating MC from adapted kubelet service file")
	u := uuid.NewUUID()
	ignConfig := ctrlcommon.NewIgnConfig()
	ignFile := helpers.CreateEncodedIgn3File(filename, data, 420)
	ignConfig.Storage.Files = append(ignConfig.Storage.Files, ignFile)
	rawIgnConfig := helpers.MarshalOrDie(ignConfig)

	return &mcfgv1.MachineConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: mcfgv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      helpers.MCLabelForRole(role),
			Annotations: map[string]string{},
			UID:         u,
		},
		Spec: mcfgv1.MachineConfigSpec{
			Config: runtime.RawExtension{
				Raw: rawIgnConfig,
			},
		},
	}
}

// getMCName returns the current configuration name of the machine config pool poolName
// Adapted from https://github.com/openshift/machine-config-operator/blob/1ac641c0d03/test/helpers/utils.go#L189
func getMCName(oc *exutil.CLI, poolName string) string {
	// grab the initial machineconfig used by the worker pool
	// this MC is gonna be the one which is going to be reapplied once the previous MC is deleted
	mcp, err := oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), poolName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	return mcp.Status.Configuration.Name
}

// waitForMC gets the rendered config for pool pool and MC name mcName, then waits for the pool to complete.
// Adapted from https://github.com/openshift/machine-config-operator/blob/1ac641c0d03/test/helpers/utils.go#L200
func waitForMC(oc *exutil.CLI, pool string, mcName string) {
	var renderedConfig string
	g.By(fmt.Sprintf("Waiting for MC %s to be rendered in pool %s", mcName, pool))
	if err := wait.PollUntilContextTimeout(context.TODO(), 2*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		// Set up the list

		// Update found based on the MCP
		mcp, err := oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), pool, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, mc := range mcp.Spec.Configuration.Source {
			if mc.Name == mcName {
				renderedConfig = mcp.Spec.Configuration.Name
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		o.Expect(err).NotTo(o.HaveOccurred())
	}
	o.Expect(renderedConfig).NotTo(o.BeEmpty())
	waitForPoolComplete(oc, pool, renderedConfig)
}

func waitForPoolComplete(oc *exutil.CLI, pool string, renderedConfig string) {
	g.By(fmt.Sprintf("Waiting for pool %s to render with MC %s", pool, renderedConfig))
	o.Expect(wait.PollUntilContextTimeout(context.TODO(), 2*time.Second, 20*time.Minute, false, func(ctx context.Context) (bool, error) {
		mcp, err := oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigPools().Get(ctx, pool, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if mcp.Status.Configuration.Name != renderedConfig {
			return false, nil
		}

		// Adapted from github.com/openshift/machine-config-operator:pkg/apihelpers/apihelpers.go
		// Vendoring it messed up, so I copied instead.
		for _, condition := range mcp.Status.Conditions {
			if condition.Type == mcfgv1.MachineConfigPoolUpdated {
				return condition.Status == corev1.ConditionTrue, nil
			}
		}

		return false, nil
	})).NotTo(o.HaveOccurred())
}

// createMCP create a machine config pool with name mcpName
// it will also use mcpName as the label selector, so any node you want to be included
// in the pool should have a label node-role.kubernetes.io/mcpName = ""
// Adapted from https://github.com/openshift/machine-config-operator/blob/1ac641c0d03/test/e2e/mcd_test.go#L707
func createMCP(oc *exutil.CLI, mcpName string) func() {
	mcp := &mcfgv1.MachineConfigPool{}
	mcp.Name = mcpName
	nodeSelector := metav1.LabelSelector{}
	mcp.Spec.NodeSelector = &nodeSelector
	mcp.Spec.NodeSelector.MatchLabels = make(map[string]string)
	mcp.Spec.NodeSelector.MatchLabels[mcpNameToRole(mcpName)] = ""
	mcSelector := metav1.LabelSelector{}
	mcp.Spec.MachineConfigSelector = &mcSelector
	mcp.Spec.MachineConfigSelector.MatchExpressions = []metav1.LabelSelectorRequirement{
		{
			Key:      mcfgv1.MachineConfigRoleLabelKey,
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{workerMCPName, mcpName},
		},
	}
	mcp.ObjectMeta.Labels = make(map[string]string)
	mcp.ObjectMeta.Labels[mcpName] = ""

	g.By("Creating MCP " + mcpName)
	_, err := oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigPools().Create(context.TODO(), mcp, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	return func() {
		g.By("Deleting MCP " + mcpName)
		o.Expect(oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigPools().Delete(context.TODO(), mcpName, metav1.DeleteOptions{})).NotTo(o.HaveOccurred())
	}
}

// labelRandomNodeFromPool gets all nodes in pool and chooses one at random to label
// Adapted from https://github.com/openshift/machine-config-operator/blob/1ac641c0d03/test/helpers/utils.go#L602
func labelRandomNodeFromPool(oc *exutil.CLI, newMCPName, oldMCPName, newMCName, oldMCName string) func() {
	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{mcpNameToRole(oldMCPName): ""}).String(),
	}
	nodes, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().List(context.TODO(), listOptions)
	o.Expect(err).NotTo(o.HaveOccurred())

	// Disable gosec here to avoid throwing
	// G404: Use of weak random number generator (math/rand instead of crypto/rand)
	// #nosec
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	node := nodes.Items[rnd.Intn(len(nodes.Items))]
	label := mcpNameToRole(newMCPName)

	o.Expect(retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		n, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		n.Labels[label] = ""
		_, err = oc.AsAdmin().KubeClient().CoreV1().Nodes().Update(context.TODO(), n, metav1.UpdateOptions{})
		return err
	})).NotTo(o.HaveOccurred())

	g.By(fmt.Sprintf("Applied label %q to node %s", label, node.Name))
	// wait for mc to rollout
	waitForMC(oc, newMCPName, newMCName)

	return func() {
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			n, err := oc.AsAdmin().KubeClient().CoreV1().Nodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			delete(n.Labels, label)
			_, err = oc.AsAdmin().KubeClient().CoreV1().Nodes().Update(context.TODO(), n, metav1.UpdateOptions{})
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By(fmt.Sprintf("Removed label %q from node %s", label, node.Name))

		waitForPoolComplete(oc, oldMCPName, oldMCName)
	}
}

// mcpNameToRole converts a mcpName to a node role label
// Adapted from https://github.com/openshift/machine-config-operator/blob/1ac641c0d03/test/helpers/utils.go#L756
func mcpNameToRole(mcpName string) string {
	return fmt.Sprintf("node-role.kubernetes.io/%s", mcpName)
}
