package ginkgo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// machineAPINamespace is the namespace Machine API resources live in.
	machineAPINamespace = "openshift-machine-api"
	// machineSetOwningLabel is stamped by the machine-api on every Machine
	// created by a MachineSet, pointing back at the owning MachineSet's name.
	machineSetOwningLabel = "machine.openshift.io/cluster-api-machineset"
	// machineRoleLabel identifies the role (worker/master) a MachineSet's
	// machines are intended to fill.
	machineRoleLabel = "machine.openshift.io/cluster-api-machine-role"

	// nodeResourcePoolLabelKey marks pool nodes for visibility (`oc get nodes -l
	// noderesource.test.openshift.io/pool`). Pool nodes are plain workers, not tainted.
	nodeResourcePoolLabelKey   = "noderesource.test.openshift.io/pool"
	nodeResourcePoolLabelValue = "static"

	// nodeResourcePoolNamePrefix plus a random suffix avoids MachineSet name collisions.
	nodeResourcePoolNamePrefix = "openshift-tests-noderesource-"

	// defaultNodeResourcePoolSize matches the worker count HA clusters were tuned for.
	defaultNodeResourcePoolSize = 3
	// nodeResourcePoolSizeEnvVar overrides pool size (OPENSHIFT_TESTS_NODERESOURCE_POOL_SIZE).
	nodeResourcePoolSizeEnvVar = "OPENSHIFT_TESTS_NODERESOURCE_POOL_SIZE"

	// nodeResourcePoolReadyTimeout covers VM boot + ignition + MCO (often 15-20m).
	nodeResourcePoolReadyTimeout = 25 * time.Minute
	nodeResourcePoolPollInterval = 15 * time.Second
)

// errMachineAPIUnavailable means no worker MachineSet to clone; skip the bucket.
var errMachineAPIUnavailable = errors.New("machine API is not available on this cluster")

// nodeResourcePool tracks the dedicated MachineSet created to back
// [NodeResource] tests and the nodes it produced, so the caller can tear it
// down once those tests finish.
type nodeResourcePool struct {
	machineSetName string
	nodeNames      []string
}

// nodeResourcePoolSize returns the number of nodes to provision for the
// dedicated NodeResource pool, defaulting to defaultNodeResourcePoolSize.
func nodeResourcePoolSize() int {
	raw := os.Getenv(nodeResourcePoolSizeEnvVar)
	if raw == "" {
		return defaultNodeResourcePoolSize
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		logrus.Warnf("Ignoring invalid %s=%q, using default of %d", nodeResourcePoolSizeEnvVar, raw, defaultNodeResourcePoolSize)
		return defaultNodeResourcePoolSize
	}
	return n
}

// createStaticNodeResourcePool clones a worker MachineSet and waits for `size`
// Ready nodes. Caller must call teardownNodeResourcePool on a non-nil pool.
func createStaticNodeResourcePool(ctx context.Context, restConfig *rest.Config, kubeClient kubernetes.Interface, size int) (*nodeResourcePool, error) {
	machineClient, err := machineclient.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create machine API client: %w", err)
	}

	template, err := findTemplateWorkerMachineSet(ctx, machineClient)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errMachineAPIUnavailable
		}
		return nil, err
	}

	poolMachineSet := buildPoolMachineSet(template, size)

	logrus.Infof("Creating dedicated NodeResource worker pool MachineSet %s/%s with %d replicas (cloned from %s)",
		poolMachineSet.Namespace, poolMachineSet.Name, size, template.Name)
	created, err := machineClient.MachineV1beta1().MachineSets(machineAPINamespace).Create(ctx, poolMachineSet, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create NodeResource pool MachineSet: %w", err)
	}

	pool := &nodeResourcePool{machineSetName: created.Name}

	nodeNames, err := waitForNodeResourcePoolReady(ctx, machineClient, kubeClient, created.Name, size)
	if err != nil {
		return pool, fmt.Errorf("NodeResource pool %s did not become ready: %w", created.Name, err)
	}
	pool.nodeNames = nodeNames

	logrus.Infof("NodeResource pool %s ready with %d node(s): %v", created.Name, len(nodeNames), nodeNames)
	return pool, nil
}

// findTemplateWorkerMachineSet picks the lexicographically first worker
// MachineSet so cloning stays platform-agnostic (AWS/GCP/Azure/etc.).
func findTemplateWorkerMachineSet(ctx context.Context, machineClient machineclient.Interface) (*machinev1beta1.MachineSet, error) {
	machineSets, err := machineClient.MachineV1beta1().MachineSets(machineAPINamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list machinesets: %w", err)
	}

	var candidates []machinev1beta1.MachineSet
	for _, ms := range machineSets.Items {
		if ms.Spec.Template.Labels[machineRoleLabel] == "worker" {
			candidates = append(candidates, ms)
		}
	}
	if len(candidates) == 0 {
		// No worker MachineSet (e.g. UPI) — treat like Machine API unavailable.
		return nil, fmt.Errorf("no worker MachineSet found in namespace %s to use as a NodeResource pool template: %w", machineAPINamespace, errMachineAPIUnavailable)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Name < candidates[j].Name })
	return &candidates[0], nil
}

// buildPoolMachineSet clones template as a plain additional worker MachineSet
// (same MCP, no taints) with a pool tracking label on resulting nodes.
func buildPoolMachineSet(template *machinev1beta1.MachineSet, size int) *machinev1beta1.MachineSet {
	pool := template.DeepCopy()

	name := nodeResourcePoolNamePrefix + rand.String(5)
	replicas := int32(size)

	pool.ObjectMeta = metav1.ObjectMeta{
		Name:      name,
		Namespace: machineAPINamespace,
	}
	pool.Status = machinev1beta1.MachineSetStatus{}

	pool.Spec.Replicas = &replicas
	pool.Spec.Selector = metav1.LabelSelector{
		MatchLabels: map[string]string{machineSetOwningLabel: name},
	}

	if pool.Spec.Template.Labels == nil {
		pool.Spec.Template.Labels = map[string]string{}
	}
	pool.Spec.Template.Labels[machineSetOwningLabel] = name

	// MachineSpec.ObjectMeta labels propagate to the Node object.
	if pool.Spec.Template.Spec.Labels == nil {
		pool.Spec.Template.Spec.Labels = map[string]string{}
	}
	pool.Spec.Template.Spec.Labels[nodeResourcePoolLabelKey] = nodeResourcePoolLabelValue

	// Clear providerID so the machine-api provisions a fresh machine.
	pool.Spec.Template.Spec.ProviderID = nil
	pool.Spec.Template.Spec.LifecycleHooks = machinev1beta1.LifecycleHooks{}

	return pool
}

// waitForNodeResourcePoolReady blocks until `expected` nodes owned by
// machineSetName are Ready, or nodeResourcePoolReadyTimeout elapses.
func waitForNodeResourcePoolReady(ctx context.Context, machineClient machineclient.Interface, kubeClient kubernetes.Interface, machineSetName string, expected int) ([]string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, nodeResourcePoolReadyTimeout)
	defer cancel()

	ticker := time.NewTicker(nodeResourcePoolPollInterval)
	defer ticker.Stop()

	for {
		names, err := readyNodeNamesForMachineSet(timeoutCtx, machineClient, kubeClient, machineSetName)
		switch {
		case err != nil:
			logrus.Warnf("Error checking readiness of NodeResource pool %s, will retry: %v", machineSetName, err)
		case len(names) >= expected:
			return names[:expected], nil
		default:
			logrus.Infof("Waiting for NodeResource pool %s: %d/%d node(s) Ready", machineSetName, len(names), expected)
		}

		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("timed out after %s waiting for %d node(s) to become Ready", nodeResourcePoolReadyTimeout, expected)
		case <-ticker.C:
		}
	}
}

// readyNodeNamesForMachineSet returns the names of Ready nodes backed by
// Machines owned by machineSetName.
func readyNodeNamesForMachineSet(ctx context.Context, machineClient machineclient.Interface, kubeClient kubernetes.Interface, machineSetName string) ([]string, error) {
	machines, err := machineClient.MachineV1beta1().Machines(machineAPINamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", machineSetOwningLabel, machineSetName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list machines for machineset %s: %w", machineSetName, err)
	}

	var ready []string
	for _, m := range machines.Items {
		if m.Status.NodeRef == nil || m.Status.NodeRef.Name == "" {
			// Machine hasn't been linked to a Node yet.
			continue
		}
		node, err := kubeClient.CoreV1().Nodes().Get(ctx, m.Status.NodeRef.Name, metav1.GetOptions{})
		if err != nil {
			// Node object may not exist/be visible yet.
			continue
		}
		if isNodeReady(node) {
			ready = append(ready, node.Name)
		}
	}
	sort.Strings(ready)
	return ready, nil
}

// teardownNodeResourcePool deletes the pool MachineSet; node removal is async.
func teardownNodeResourcePool(ctx context.Context, restConfig *rest.Config, pool *nodeResourcePool) {
	if pool == nil || pool.machineSetName == "" {
		return
	}

	machineClient, err := machineclient.NewForConfig(restConfig)
	if err != nil {
		logrus.Errorf("Failed to create machine API client to tear down NodeResource pool %s: %v", pool.machineSetName, err)
		return
	}

	logrus.Infof("Deleting NodeResource pool MachineSet %s (%d node(s))", pool.machineSetName, len(pool.nodeNames))
	if err := machineClient.MachineV1beta1().MachineSets(machineAPINamespace).Delete(ctx, pool.machineSetName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		logrus.Errorf("Failed to delete NodeResource pool MachineSet %s: %v", pool.machineSetName, err)
	}
}
