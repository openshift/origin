package networking

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"

	"github.com/openshift-kni/commatrix/pkg/client"
	"k8s.io/client-go/tools/clientcmd"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift-kni/commatrix/pkg/consts"
	"github.com/openshift-kni/commatrix/pkg/types"
	"github.com/openshift-kni/commatrix/pkg/utils"
	exutil "github.com/openshift/origin/test/extended/util"

	commatrixcreator "github.com/openshift-kni/commatrix/pkg/commatrix-creator"
	"github.com/openshift-kni/commatrix/pkg/endpointslices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("[sig-network][Feature:commatrix][Serial]", func() {
	g.It("should apply firewall by blocking all ports except the ones OCP is listening on", func() {
		g.By("generating the commatrix")

		cs, err := client.New()
		if err != nil {
			panic(fmt.Errorf("failed creating the k8s client: %w", err))
		}

		deployment := types.MNO
		isSNO, err := isSNOCluster(cs)
		o.Expect(err).NotTo(o.HaveOccurred())

		if isSNO {
			deployment = types.SNO
		}

		infra, err := infrtType()
		o.Expect(err).NotTo(o.HaveOccurred())

		epExporter, err := endpointslices.New(cs)
		if err != nil {
			panic(fmt.Errorf("failed creating the endpointslices exporter: %w", err))
		}

		commMatrix, err := commatrixcreator.New(epExporter, "", "", infra, deployment)
		o.Expect(err).NotTo(o.HaveOccurred())

		matrix, err := commMatrix.CreateEndpointMatrix()
		o.Expect(err).NotTo(o.HaveOccurred())

		masterMat, workerMat := matrix.SeparateMatrixByRole()

		masterNFT, err := masterMat.ToNFTables()
		o.Expect(err).NotTo(o.HaveOccurred())

		workerNFT, err := workerMat.ToNFTables()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("apply firewall on master nodes")
		err = applyRulesToNode(cs, "master", masterNFT)
		o.Expect(err).ToNot(o.HaveOccurred())

		if !isSNO {
			g.By("apply firewall on worker nodes")
			err = applyRulesToNode(cs, "worker", workerNFT)
			o.Expect(err).ToNot(o.HaveOccurred())
		}
	})
})

func applyRulesToNode(cs *client.ClientSet, role string, NFTtable []byte) error {
	utilsHelpers := utils.New(cs)

	nodesList, err := cs.CoreV1Interface.Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	artifactsDir := filepath.Join(exutil.ArtifactDirPath(), "commatrix")
	err = os.MkdirAll(artifactsDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create artifactsDir %w", err)
	}

	for _, node := range nodesList.Items {
		nodeRolde, err := types.GetNodeRole(&node)
		if err != nil {
			return err
		}
		if nodeRolde != role {
			continue
		}
		ns := "commatrix-firewall"
		err = utilsHelpers.CreateNamespace(ns)
		if err != nil {
			return err
		}

		defer func() error {
			err := utilsHelpers.DeleteNamespace(ns)
			if err != nil {
				return err
			}
			return nil
		}()

		debugPod, err := utilsHelpers.CreatePodOnNode(node.Name, ns, consts.DefaultDebugPodImage)
		if err != nil {
			return fmt.Errorf("failed to create debug pod on node %s: %w", node.Name, err)
		}

		defer func() {
			err := utilsHelpers.DeletePod(debugPod)
			if err != nil {
				fmt.Printf("failed cleaning debug pod %s: %v", debugPod, err)
			}
		}()

		command := []string{
			"bash", "-c",
			fmt.Sprintf("mkdir -p nft && echo '%s' > nft/firewall.nft", string(NFTtable)),
		}
		_, err = utilsHelpers.RunCommandOnPod(debugPod, command)
		if err != nil {
			return fmt.Errorf("failed to save rule set to file on node %s: %w", node.Name, err)
		}

		installCmd := []string{"bash", "-c", "yum update -y && yum install -y nftables"}
		_, err = utilsHelpers.RunCommandOnPod(debugPod, installCmd)
		if err != nil {
			return fmt.Errorf("failed to save rule set to file on node %s: %w", node.Name, err)
		}

		command = []string{
			"bash", "-c", "/usr/sbin/nft -f nft/firewall.nft",
		}
		_, err = utilsHelpers.RunCommandOnPod(debugPod, command)
		if err != nil {
			return fmt.Errorf("failed to apply rule set %q on node %s: %w", command, node.Name, err)
		}

		// Check the output of nft list ruleset
		command = []string{"bash", "-c", "/usr/sbin/nft list ruleset"}
		output, err := utilsHelpers.RunCommandOnPod(debugPod, command)
		if err != nil {
			return fmt.Errorf("failed to list NFT ruleset on node %s: %w", node.Name, err)
		}
		fmt.Printf("Command output: %s\n", string(output))

		if len(output) == 0 {
			return fmt.Errorf("no output from 'nft list ruleset' on node %s: ", node.Name)
		}
		command = []string{
			"bash", "-c",
			"/usr/sbin/nft list ruleset > /etc/nftables.conf",
		}

		_, err = utilsHelpers.RunCommandOnPod(debugPod, command)
		if err != nil {
			return fmt.Errorf("failed to save NFT ruleset to file on node %s: %w", node.Name, err)
		}

		command = []string{"bash", "-c",
			"systemctl enable nftables",
		}
		_, err = utilsHelpers.RunCommandOnPod(debugPod, command)
		if err != nil {
			return fmt.Errorf("failed to enable nftables %q on node %s: %w", command, node.Name, err)
		}

		// save the nft ruleset on ArtifactDirPath
		err = os.WriteFile(filepath.Join(artifactsDir, fmt.Sprintf("nftables-%s.nft", node.Name)), []byte(output), 0644)
		if err != nil {
			return fmt.Errorf("failed to save nft file on for node artifacts %s %w", node.Name, err)
		}
	}
	return nil
}

// isSNOCluster will check if OCP is a single node cluster
func isSNOCluster(cs *client.ClientSet) (bool, error) {
	nodes, err := cs.CoreV1Interface.Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	if len(nodes.Items) > 1 {
		return false, nil
	}
	return true, nil
}

func infrtType() (types.Env, error) {
	kubeconfig, ok := os.LookupEnv("KUBECONFIG")
	if !ok {
		g.Fail("must set the KUBECONFIG environment variable")
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return types.Cloud, err
	}
	oc := configv1client.NewForConfigOrDie(config)
	infra, err := oc.Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return -1, err
	}
	switch infra.Status.PlatformStatus.Type {
	case configv1.BareMetalPlatformType:
		return types.Baremetal, nil
	case configv1.OpenStackPlatformType:
		return types.Cloud, nil
	case configv1.VSpherePlatformType:
		return types.Cloud, nil
	case configv1.AWSPlatformType:
		return types.Cloud, nil
	case configv1.AzurePlatformType:
		return types.Cloud, nil
	case configv1.GCPPlatformType:
		return -1, fmt.Errorf("un supported platform")
	case configv1.NonePlatformType:
		return -1, fmt.Errorf("un supported platform")
	default:
		return -1, fmt.Errorf("no supported platform detected")
	}
}
