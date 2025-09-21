package cluster

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/onsi/gomega"
	"github.com/openshift-kni/commatrix/pkg/client"

	machineconfigurationv1 "github.com/openshift/api/machineconfiguration/v1"
	ocpoperatorv1 "github.com/openshift/api/operator/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	mcoac "github.com/openshift/client-go/operator/applyconfigurations/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	controllersClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

const (
	thresholdVersion = "4.16" // above this version the nft service must be added to the NodeDisruptionPolicy in the MachineConfiguration.
	timeout          = 20 * time.Minute
	interval         = 5 * time.Second
)

// getClusterVersion return cluster's Y stream version.
func GetClusterVersion(cs *client.ClientSet) (string, error) {
	configClient := configv1client.NewForConfigOrDie(cs.Config)
	clusterVersion, err := configClient.ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	clusterVersionParts := strings.SplitN(clusterVersion.Status.Desired.Version, ".", 3)
	return strings.Join(clusterVersionParts[:2], "."), nil
}

func WaitForMCPUpdateToStart(cs *client.ClientSet, role string) {
	gomega.Eventually(func() (bool, error) {
		mcp := &machineconfigurationv1.MachineConfigPool{}
		err := cs.Get(context.TODO(), controllersClient.ObjectKey{Name: role}, mcp)
		if err != nil {
			return false, fmt.Errorf("failed to get %s MachineConfigPool: %v", role, err)
		}

		if mcp.Status.UpdatedMachineCount != mcp.Status.MachineCount {
			log.Printf("MCP %s has started updating", mcp.Name)
			return true, nil
		}

		return false, nil
	}, timeout, 30*time.Second).Should(gomega.BeTrue(), "Timed out waiting for MCP to start updating")
}

func AddNFTSvcToNodeDisruptionPolicy(cs *client.ClientSet) error {
	machineConfigurationClient := cs.MCInterface
	reloadApplyConfiguration := mcoac.ReloadService().WithServiceName("nftables.service")
	restartApplyConfiguration := mcoac.RestartService().WithServiceName("nftables.service")

	serviceName := "nftables.service"
	serviceApplyConfiguration := mcoac.NodeDisruptionPolicySpecUnit().WithName(ocpoperatorv1.NodeDisruptionPolicyServiceName(serviceName)).WithActions(
		mcoac.NodeDisruptionPolicySpecAction().WithType(ocpoperatorv1.ReloadSpecAction).WithReload(reloadApplyConfiguration),
	)
	fileApplyConfiguration := mcoac.NodeDisruptionPolicySpecFile().WithPath("/etc/sysconfig/nftables.conf").WithActions(
		mcoac.NodeDisruptionPolicySpecAction().WithType(ocpoperatorv1.RestartSpecAction).WithRestart(restartApplyConfiguration),
	)

	applyConfiguration := mcoac.MachineConfiguration("cluster").WithSpec(mcoac.MachineConfigurationSpec().
		WithManagementState("Managed").WithNodeDisruptionPolicy(mcoac.NodeDisruptionPolicyConfig().
		WithUnits(serviceApplyConfiguration).WithFiles(fileApplyConfiguration)))

	_, err := machineConfigurationClient.OperatorV1().MachineConfigurations().Apply(context.TODO(), applyConfiguration,
		metav1.ApplyOptions{FieldManager: "machine-config-operator", Force: true})
	if err != nil {
		return fmt.Errorf("updating cluster node disruption policy failed %v", err)
	}

	log.Println("MachineConfiguration updated successfully!")
	return nil
}

// ApplyMachineConfig applies the MachineConfig and returns true if created or updated.
// False if unchanged, and an error if there is error.
func ApplyMachineConfig(yamlInput []byte, c *client.ClientSet) (bool, error) {
	obj := &machineconfigurationv1.MachineConfig{}
	if err := yaml.Unmarshal(yamlInput, obj); err != nil {
		return false, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	modifiedConfig := obj.Spec.Config.DeepCopy()
	operationResult, err := controllerutil.CreateOrUpdate(context.TODO(), c, obj, func() error {
		obj.Spec.Config = *modifiedConfig
		return nil
	})

	if err != nil {
		return false, fmt.Errorf("failed to apply MachineConfig: %w", err)
	}

	if operationResult == controllerutil.OperationResultNone {
		return false, nil
	}

	return true, nil
}

func WaitForMCPReadyState(c *client.ClientSet, role string) {
	gomega.Eventually(func() (bool, error) {
		mcp := &machineconfigurationv1.MachineConfigPool{}
		err := c.Get(context.TODO(), controllersClient.ObjectKey{Name: role}, mcp)
		if err != nil {
			return false, fmt.Errorf("failed to get %s MachineConfigPool: %v", role, err)
		}

		if mcp.Status.ReadyMachineCount != mcp.Status.MachineCount {
			log.Printf("MCP %s is still updating or degraded\n", mcp.Name)
			return false, nil
		}

		log.Println("All MCPs are ready and updated")
		return true, nil
	}, timeout, 30*time.Second).Should(gomega.BeTrue(), "Timed out waiting for MCPs to reach the ready state")
}

func ValidateClusterVersionAndMachineConfiguration(cs *client.ClientSet) error {
	thresholdVersionSemver := semver.MustParse(thresholdVersion)
	clusterVersion, err := GetClusterVersion(cs)
	if err != nil {
		return err
	}

	currentVersion, err := semver.NewVersion(clusterVersion)
	if err != nil {
		return err
	}

	if currentVersion.GreaterThan(thresholdVersionSemver) {
		log.Printf("Version Greater Than " + thresholdVersion + " - Updating Machine Configuration")
		err = AddNFTSvcToNodeDisruptionPolicy(cs)
		if err != nil {
			return err
		}
	}

	return nil
}

func WaitForAPIGroupAvailable(cs *client.ClientSet, groupVersion string) {
	disco := discovery.NewDiscoveryClientForConfigOrDie(cs.Config)
	gomega.Eventually(func() bool {
		_, err := disco.ServerResourcesForGroupVersion(groupVersion)
		return err == nil
	}, timeout, interval).Should(gomega.BeTrue(), "API group %s not available", groupVersion)
}
