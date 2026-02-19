package logcollectorservice

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	mcfgclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

const (
	logCollectorPort = 9333
)

// FetchLogsFromService retrieves logs from the log collector service running on a node.
// This function can be called even when kubelet is down because the service starts
// before kubelet and listens on port 9333.
func FetchLogsFromService(ctx context.Context, kubeClient kubernetes.Interface, nodeName, logType string) (string, error) {
	// Get the node to find its internal IP
	node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	var nodeIP string
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			nodeIP = addr.Address
			break
		}
	}

	if nodeIP == "" {
		return "", fmt.Errorf("no internal IP found for node %s", nodeName)
	}

	// Construct the URL based on log type
	var endpoint string
	switch logType {
	case "kubelet":
		endpoint = fmt.Sprintf("http://%s:%d/logs/kubelet", nodeIP, logCollectorPort)
	case "crio":
		endpoint = fmt.Sprintf("http://%s:%d/logs/crio", nodeIP, logCollectorPort)
	case "both":
		endpoint = fmt.Sprintf("http://%s:%d/logs/both", nodeIP, logCollectorPort)
	default:
		return "", fmt.Errorf("unknown log type: %s (valid: kubelet, crio, both)", logType)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to fetch logs from %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("log collector service returned status %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

// IsServiceHealthy checks if the log collector service is running on a node
func IsServiceHealthy(ctx context.Context, kubeClient kubernetes.Interface, nodeName string) bool {
	node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return false
	}

	var nodeIP string
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			nodeIP = addr.Address
			break
		}
	}

	if nodeIP == "" {
		return false
	}

	healthURL := fmt.Sprintf("http://%s:%d/health", nodeIP, logCollectorPort)
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(healthURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// EnsureLogCollectorForPool deploys the log collector service to a specific MachineConfigPool.
// This function can be called from tests after they create custom MachineConfigPools.
// It's idempotent - if the MachineConfig already exists, it will skip creation.
func EnsureLogCollectorForPool(ctx context.Context, adminRESTConfig *rest.Config, poolName string) error {
	mcfgClient, err := mcfgclient.NewForConfig(adminRESTConfig)
	if err != nil {
		return fmt.Errorf("failed to create machine config client: %w", err)
	}

	// Load the template MachineConfig from the YAML file
	mcPath := exutil.FixturePath("testdata", "machine_config", "machineconfig", "99-node-log-collector.yaml")

	data, err := ioutil.ReadFile(mcPath)
	if err != nil {
		return fmt.Errorf("failed to read MachineConfig file %s: %w", mcPath, err)
	}

	// Parse the YAML into a MachineConfig object
	var mc mcfgv1.MachineConfig
	if err := yaml.Unmarshal(data, &mc); err != nil {
		return fmt.Errorf("failed to unmarshal MachineConfig: %w", err)
	}

	// Customize the MachineConfig for this pool
	mcName := fmt.Sprintf("99-%s-node-log-collector", poolName)
	mc.ObjectMeta.Name = mcName
	mc.ObjectMeta.Labels = map[string]string{
		"machineconfiguration.openshift.io/role": poolName,
	}

	// Check if MachineConfig already exists
	existing, err := mcfgClient.MachineconfigurationV1().MachineConfigs().Get(ctx, mcName, metav1.GetOptions{})
	if err == nil {
		// MachineConfig already exists
		fmt.Printf("MachineConfig %s already exists (created at %v), skipping creation\n",
			mcName, existing.CreationTimestamp)
		return nil
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check if MachineConfig exists: %w", err)
	}

	// Create the MachineConfig
	fmt.Printf("Creating MachineConfig %s for pool %s on port %d\n",
		mcName, poolName, logCollectorPort)

	_, err = mcfgClient.MachineconfigurationV1().MachineConfigs().Create(
		ctx,
		&mc,
		metav1.CreateOptions{},
	)

	if err != nil {
		return fmt.Errorf("failed to create MachineConfig: %w", err)
	}

	fmt.Printf("Successfully created MachineConfig %s for pool %s\n", mcName, poolName)

	// Wait for the MachineConfigPool to start updating
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			mcp, err := mcfgClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			// Check if MCP is updating
			for _, condition := range mcp.Status.Conditions {
				if condition.Type == "Updating" && condition.Status == "True" {
					fmt.Printf("MachineConfigPool '%s' is now updating\n", poolName)
					return true, nil
				}
			}

			return false, nil
		})

	if err != nil {
		// Don't fail on timeout - the MCP might update later
		fmt.Printf("MachineConfigPool '%s' update wait timed out (this is OK): %v\n", poolName, err)
	}

	return nil
}
