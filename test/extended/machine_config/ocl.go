package machine_config

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	mcfgv1alpha1 "github.com/openshift/api/machineconfiguration/v1alpha1"
	ctrlcommon "github.com/openshift/machine-config-operator/pkg/controller/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mcClient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"

	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	MCOMachineConfigBaseDir = exutil.FixturePath("testdata", "machine_config")
	oc                      = exutil.NewCLIWithoutNamespace("machine-config")
)

var _ = g.BeforeSuite(func() {

	// Create input pull secret
	inputPullSecretName := "my-input-pull"
	ps, err := oc.AsAdmin().AdminKubeClient().CoreV1().Secrets("openshift-config").Get(context.TODO(), "pull-secret", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Get pull-secret from openshift-config")
	localInputPullSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: inputPullSecretName,
		},
		Data: ps.Data,
		Type: ps.Type,
	}
	_, err = oc.KubeClient().CoreV1().Secrets(mcoNamespace).Create(context.TODO(), localInputPullSecret, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Create my-input-pull secret in openshift-machine-config")

	// Create service account toke for the builder
	longLivedTokenName := "long-live-token"
	serviceAccountName := "builder"
	localTokenPullSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      longLivedTokenName,
			Namespace: mcoNamespace,
			Annotations: map[string]string{
				corev1.ServiceAccountNameKey: serviceAccountName,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}
	_, err = oc.KubeClient().CoreV1().Secrets(mcoNamespace).Create(context.Background(), localTokenPullSecret, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Create a long-live-token secret in openshift-machine-config")
})

var _ = g.Describe("[sig-mco][OCPFeatureGate:OnClusterBuild]", func() {
	defer g.GinkgoRecover()
	var (
		moscFixture = filepath.Join(MCOMachineConfigBaseDir, "machineosconfigurations", "machineosconfig.yaml")
		mcpFixture  = filepath.Join(MCOMachineConfigBaseDir, "machineconfigpool", "machineconfigpool.yaml")
	)

	g.It("Should update opted in MCP with the build image from the dockerfile mentioned in MOSC [apigroup:machineconfiguration.openshift.io]", func() {
		AllNodePoolOptInTest(oc, moscFixture, mcpFixture)
	})

})

func AllNodePoolOptInTest(oc *exutil.CLI, moscFixture string, mcpFixture string) {

	err := oc.Run("apply").Args("-f", mcpFixture).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), "Create MCP Infra")

	nodes, err := oc.KubeClient().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Get all nodes")
	for _, node := range nodes.Items {
		err = oc.AsAdmin().Run("label").Args("node", node.Name, "node-role.kubernetes.io/infra="+"").Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Add node %s to MCP infra", node.Name))
	}
	err = oc.Run("apply").Args("-f", moscFixture).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), "Create MOSC Infra and opt in Infra pool into OCL")

	machineConfigClient, err := mcClient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(err).NotTo(o.HaveOccurred())

	mcp, err := getMCPFromFixture(mcpFixture)
	o.Expect(err).NotTo(o.HaveOccurred())

	mosc, err := getMOSCFromFixture(moscFixture)
	o.Expect(err).NotTo(o.HaveOccurred())

	waitTime := time.Minute * 20
	ctx, cancel := context.WithTimeout(context.Background(), waitTime)
	defer cancel()

	// Wait for MOSB to be created
	err = waitForBuild(ctx, machineConfigClient, mcp)
	o.Expect(err).NotTo(o.HaveOccurred(), "Waiting for MOSB to be created and builder pod to Succeed")

	// Wait for the build Image to be applied to mcp successfully
	err = waitForRollout(ctx, machineConfigClient, oc, mcp, mosc)
	o.Expect(err).NotTo(o.HaveOccurred(), "Waiting for MCP to have desired config as the new built image")
}

func waitForRollout(ctx context.Context, clientset *mcClient.Clientset, oc *exutil.CLI, mcpG *mcfgv1.MachineConfigPool, moscG *mcfgv1alpha1.MachineOSConfig) error {
	return wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (done bool, err error) {

		//Get mosb
		var mosb mcfgv1alpha1.MachineOSBuild
		var mosbFound bool
		mosbList, err := clientset.MachineconfigurationV1alpha1().MachineOSBuilds().List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		for _, m := range mosbList.Items {
			if m.Spec.DesiredConfig.Name == mcpG.Spec.Configuration.Name {
				mosb = m
				mosbFound = true
				break
			}
		}
		if !mosbFound {
			return false, fmt.Errorf("could not find mosb")
		}

		// Get mosc
		mosc, err := clientset.MachineconfigurationV1alpha1().MachineOSConfigs().Get(ctx, moscG.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("couldnt get mosc: %w", err)
		}

		// Get Pool
		pool, err := clientset.MachineconfigurationV1().MachineConfigPools().Get(ctx, mcpG.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("couldnt get pool: %w", err)
		}

		// Get nodes
		nodes, err := getNodesForPool(ctx, oc, pool)

		// Ensure rollout
		doneNodes := 0
		for _, node := range nodes.Items {
			lns := ctrlcommon.NewLayeredNodeState(&node)
			if mosb.Spec.DesiredConfig.Name == pool.Spec.Configuration.Name && lns.IsCurrentImageEqualToBuild(mosc) {
				doneNodes += 1
			}
		}
		if doneNodes == len(nodes.Items) {
			return true, nil
		}
		return false, nil
	})
}

func waitForBuild(ctx context.Context, clientset *mcClient.Clientset, mcp *mcfgv1.MachineConfigPool) error {
	return wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (done bool, err error) {

		mosbList, err := clientset.MachineconfigurationV1alpha1().MachineOSBuilds().List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		isPending := false
		isBuilding := false
		isSuccess := false
		start := time.Now()

		for _, mosb := range mosbList.Items {
			if mosb.Spec.DesiredConfig.Name == mcp.Spec.Configuration.Name {
				state := ctrlcommon.NewMachineOSBuildState(&mosb)
				if !isPending && state.IsBuildPending() {
					isPending = true
					e2e.Logf("Build %s is now pending after %s", mosb.Name, time.Since(start))
				}
				if !isBuilding && state.IsBuilding() {
					isBuilding = true
					e2e.Logf("Build %s is now running after %s", mosb.Name, time.Since(start))
				}
				if !isSuccess && state.IsBuildSuccess() {
					isSuccess = true
					e2e.Logf("Build %s is complete after %s", mosb.Name, time.Since(start))
					return true, nil
				}
				if state.IsBuildFailure() {
					return false, fmt.Errorf("build %s failed after %s", mosb.Name, time.Since(start))
				}
				break
			}
		}
		return false, nil
	})
}
