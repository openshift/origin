package ginkgo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	machinev1beta1 "github.com/openshift/api/machine/v1beta1"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	machineAPINamespace = "openshift-machine-api"
	capiNamespace       = "openshift-cluster-api"

	machineSetOwningLabel = "machine.openshift.io/cluster-api-machineset"
	machineRoleLabel      = "machine.openshift.io/cluster-api-machine-role"
	machineAnnotationKey  = "machine.openshift.io/machine"
	workerNodeRoleLabel   = "node-role.kubernetes.io/worker"
	infraNodeRoleLabel    = "node-role.kubernetes.io/infra"

	capiClusterNameLabel = "cluster.x-k8s.io/cluster-name"
	capiMachineSetLabel  = "cluster.x-k8s.io/set-name"

	nodeResourcePoolLabelKey   = "noderesource.test.openshift.io/pool"
	nodeResourcePoolLabelValue = "static"
	nodeResourcePoolNamePrefix = "openshift-tests-noderesource-"

	defaultNodeResourcePoolSize = 3
	nodeResourcePoolSizeEnvVar  = "OPENSHIFT_TESTS_NODERESOURCE_POOL_SIZE"

	// nodeResourcePoolReadyTimeout covers VM boot, ignition, and MCO (often 15–20m).
	nodeResourcePoolReadyTimeout = 25 * time.Minute
	nodeResourcePoolPollInterval = 15 * time.Second
	// nodeResourcePoolTeardownTimeout bounds MachineSet deletion and machine GC.
	nodeResourcePoolTeardownTimeout = 5 * time.Minute
)

// errMachineAPIUnavailable means no worker template could be found to clone; the
// NodeResource bucket is skipped unless another provisioning path succeeds.
var errMachineAPIUnavailable = errors.New("no worker MachineSet is available on this cluster")

// nodeResourcePool tracks nodes used for NodeResource tests. When machineSetName
// is set, nodes came from a cloned MachineSet and teardown deletes that set.
type nodeResourcePool struct {
	machineSetName string
	nodeNames      []string
	capi           bool
}

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
	if n > math.MaxInt32 {
		logrus.Warnf("Ignoring %s=%d above MaxInt32, using default of %d", nodeResourcePoolSizeEnvVar, n, defaultNodeResourcePoolSize)
		return defaultNodeResourcePoolSize
	}
	return n
}

// createStaticNodeResourcePool clones a worker MachineSet (MAPI, then CAPI) and
// waits for Ready nodes. Caller must call teardownNodeResourcePool on a non-nil pool.
func createStaticNodeResourcePool(ctx context.Context, restConfig *rest.Config, kubeClient kubernetes.Interface, size int) (*nodeResourcePool, error) {
	machineClient, err := machineclient.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create machine API client: %w", err)
	}

	template, err := findTemplateWorkerMachineSet(ctx, machineClient, kubeClient)
	if err != nil {
		return nil, err
	}
	if template != nil {
		return createMAPINodeResourcePool(ctx, machineClient, kubeClient, template, size)
	}
	return createCAPINodeResourcePool(ctx, restConfig, kubeClient, size)
}

func createMAPINodeResourcePool(ctx context.Context, machineClient machineclient.Interface, kubeClient kubernetes.Interface, template *machinev1beta1.MachineSet, size int) (*nodeResourcePool, error) {
	poolMachineSet := buildPoolMachineSet(template, size)
	logrus.Infof("Creating dedicated NodeResource worker pool MachineSet %s/%s with %d replicas (cloned from %s)",
		poolMachineSet.Namespace, poolMachineSet.Name, size, template.Name)
	created, err := machineClient.MachineV1beta1().MachineSets(machineAPINamespace).Create(ctx, poolMachineSet, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create NodeResource pool MachineSet: %w", err)
	}
	pool := &nodeResourcePool{machineSetName: created.Name}
	nodeNames, err := waitForPoolNodesReady(ctx, created.Name, size, func(ctx context.Context) ([]string, error) {
		return readyNodeNamesForMachineSet(ctx, machineClient, kubeClient, created.Name)
	})
	if err != nil {
		return pool, fmt.Errorf("NodeResource pool %s did not become ready: %w", created.Name, err)
	}
	pool.nodeNames = nodeNames
	logrus.Infof("NodeResource pool %s ready with %d node(s): %v", created.Name, len(nodeNames), nodeNames)
	return pool, nil
}

// createCAPINodeResourcePool provisions the pool from openshift-cluster-api when
// TechPreview clusters have no MAPI worker MachineSet to clone.
func createCAPINodeResourcePool(ctx context.Context, restConfig *rest.Config, kubeClient kubernetes.Interface, size int) (*nodeResourcePool, error) {
	dc, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}
	template, err := findCAPITemplateWorkerMachineSet(ctx, dc)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, errMachineAPIUnavailable
	}
	poolMS := buildPoolCAPIMachineSet(template, size)
	logrus.Infof("Creating dedicated NodeResource worker pool MachineSet %s/%s with %d replicas (cloned from CAPI %s)",
		capiNamespace, poolMS.GetName(), size, template.GetName())
	created, err := dc.Resource(capiMachineSetGVR).Namespace(capiNamespace).Create(ctx, poolMS, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create NodeResource pool MachineSet: %w", err)
	}
	pool := &nodeResourcePool{machineSetName: created.GetName(), capi: true}
	nodeNames, err := waitForPoolNodesReady(ctx, created.GetName(), size, func(ctx context.Context) ([]string, error) {
		return readyNodeNamesForCAPIMachineSet(ctx, dc, kubeClient, created.GetName())
	})
	if err != nil {
		return pool, fmt.Errorf("NodeResource pool %s did not become ready: %w", created.GetName(), err)
	}
	pool.nodeNames = nodeNames
	logrus.Infof("NodeResource pool %s ready with %d node(s): %v", created.GetName(), len(nodeNames), nodeNames)
	return pool, nil
}

// findTemplateWorkerMachineSet returns a MAPI worker template, or (nil, nil) to try CAPI.
func findTemplateWorkerMachineSet(ctx context.Context, machineClient machineclient.Interface, kubeClient kubernetes.Interface) (*machinev1beta1.MachineSet, error) {
	machineSets, err := machineClient.MachineV1beta1().MachineSets(machineAPINamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list machinesets: %w", err)
	}
	var candidates []machinev1beta1.MachineSet
	for _, ms := range machineSets.Items {
		if ms.Spec.Template.Labels[machineRoleLabel] == "worker" {
			candidates = append(candidates, ms)
		}
	}
	if len(candidates) > 0 {
		sort.Slice(candidates, func(i, j int) bool { return candidates[i].Name < candidates[j].Name })
		return &candidates[0], nil
	}
	return templateWorkerMachineSetFromNode(ctx, machineClient, kubeClient)
}

// templateWorkerMachineSetFromNode resolves a MAPI worker MachineSet via a plain
// worker Node's machine.openshift.io/machine annotation (no CAPI clusters).
func templateWorkerMachineSetFromNode(ctx context.Context, machineClient machineclient.Interface, kubeClient kubernetes.Interface) (*machinev1beta1.MachineSet, error) {
	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: workerNodeRoleLabel})
	if err != nil {
		return nil, fmt.Errorf("failed to list worker nodes: %w", err)
	}
	sort.Slice(nodes.Items, func(i, j int) bool { return nodes.Items[i].Name < nodes.Items[j].Name })
	for i := range nodes.Items {
		node := &nodes.Items[i]
		if !isNodeReady(node) || node.Labels[nodeResourcePoolLabelKey] == nodeResourcePoolLabelValue {
			continue
		}
		if !isPlainWorkerNode(*node) {
			continue
		}
		namespace, name, ok := strings.Cut(node.Annotations[machineAnnotationKey], "/")
		if !ok {
			continue
		}
		machine, err := machineClient.MachineV1beta1().Machines(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("failed to get machine %s/%s: %w", namespace, name, err)
			}
			continue
		}
		machineSetName := machine.Labels[machineSetOwningLabel]
		if machineSetName == "" {
			continue
		}
		ms, err := machineClient.MachineV1beta1().MachineSets(namespace).Get(ctx, machineSetName, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("failed to get machineset %s/%s: %w", namespace, machineSetName, err)
			}
			continue
		}
		return ms, nil
	}
	return nil, nil
}

func isPlainWorkerNode(node corev1.Node) bool {
	if _, ok := node.Labels[workerNodeRoleLabel]; !ok {
		return false
	}
	if _, ok := node.Labels[infraNodeRoleLabel]; ok {
		return false
	}
	for label := range node.Labels {
		role, ok := strings.CutPrefix(label, "node-role.kubernetes.io/")
		if !ok || role == "worker" {
			continue
		}
		return false
	}
	return true
}

// findCAPITemplateWorkerMachineSet picks a worker CAPI MachineSet in openshift-cluster-api.
func findCAPITemplateWorkerMachineSet(ctx context.Context, dc dynamic.Interface) (*unstructured.Unstructured, error) {
	list, err := dc.Resource(capiMachineSetGVR).Namespace(capiNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list CAPI MachineSets: %w", err)
	}
	var candidates []*unstructured.Unstructured
	for i := range list.Items {
		ms := &list.Items[i]
		if capiMachineSetIsNodeResourcePool(ms) {
			continue
		}
		tplLabels, found, _ := unstructured.NestedMap(ms.Object, "spec", "template", "metadata", "labels")
		if !found {
			continue
		}
		if _, ok := tplLabels[workerNodeRoleLabel]; !ok {
			continue
		}
		if _, ok := tplLabels[infraNodeRoleLabel]; ok {
			continue
		}
		candidates = append(candidates, ms)
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].GetName() < candidates[j].GetName() })
	return candidates[0], nil
}

func capiMachineSetIsNodeResourcePool(ms *unstructured.Unstructured) bool {
	if ms.GetLabels()[nodeResourcePoolLabelKey] == nodeResourcePoolLabelValue {
		return true
	}
	tplLabels, found, _ := unstructured.NestedMap(ms.Object, "spec", "template", "metadata", "labels")
	if !found {
		return false
	}
	v, _ := tplLabels[nodeResourcePoolLabelKey].(string)
	return v == nodeResourcePoolLabelValue
}

// buildPoolMachineSet clones a MAPI template; pool labels propagate to Nodes via MachineSpec.ObjectMeta.
func buildPoolMachineSet(template *machinev1beta1.MachineSet, size int) *machinev1beta1.MachineSet {
	pool := template.DeepCopy()
	name := nodeResourcePoolNamePrefix + rand.String(5)
	replicas := int32(size)
	pool.ObjectMeta = metav1.ObjectMeta{Name: name, Namespace: machineAPINamespace}
	pool.Status = machinev1beta1.MachineSetStatus{}
	pool.Spec.Replicas = &replicas
	pool.Spec.Selector = metav1.LabelSelector{MatchLabels: map[string]string{machineSetOwningLabel: name}}
	if pool.Spec.Template.Labels == nil {
		pool.Spec.Template.Labels = map[string]string{}
	}
	pool.Spec.Template.Labels[machineSetOwningLabel] = name
	if pool.Spec.Template.Spec.Labels == nil {
		pool.Spec.Template.Spec.Labels = map[string]string{}
	}
	pool.Spec.Template.Spec.Labels[nodeResourcePoolLabelKey] = nodeResourcePoolLabelValue
	// Clear ProviderID so the machine-api provisions a fresh machine.
	pool.Spec.Template.Spec.ProviderID = nil
	pool.Spec.Template.Spec.LifecycleHooks = machinev1beta1.LifecycleHooks{}
	return pool
}

// buildPoolCAPIMachineSet clones a CAPI worker template with a unique set-name and pool marker.
func buildPoolCAPIMachineSet(template *unstructured.Unstructured, size int) *unstructured.Unstructured {
	pool := template.DeepCopy()
	name := nodeResourcePoolNamePrefix + rand.String(5)
	clusterName, _, _ := unstructured.NestedString(pool.Object, "spec", "clusterName")
	if clusterName == "" {
		clusterName, _, _ = unstructured.NestedString(pool.Object, "spec", "selector", "matchLabels", capiClusterNameLabel)
	}
	matchLabels := map[string]interface{}{capiClusterNameLabel: clusterName, capiMachineSetLabel: name}
	unstructured.SetNestedMap(pool.Object, matchLabels, "spec", "selector", "matchLabels")
	unstructured.SetNestedField(pool.Object, int64(size), "spec", "replicas")
	tplLabels, _, _ := unstructured.NestedMap(pool.Object, "spec", "template", "metadata", "labels")
	if tplLabels == nil {
		tplLabels = map[string]interface{}{}
	}
	tplLabels[capiClusterNameLabel] = clusterName
	tplLabels[capiMachineSetLabel] = name
	tplLabels[nodeResourcePoolLabelKey] = nodeResourcePoolLabelValue
	unstructured.SetNestedMap(pool.Object, tplLabels, "spec", "template", "metadata", "labels")
	pool.SetName(name)
	pool.SetNamespace(capiNamespace)
	pool.SetLabels(map[string]string{
		capiClusterNameLabel:     clusterName,
		nodeResourcePoolLabelKey: nodeResourcePoolLabelValue,
	})
	pool.SetResourceVersion("")
	pool.SetUID("")
	pool.SetManagedFields(nil)
	unstructured.RemoveNestedField(pool.Object, "status")
	unstructured.RemoveNestedField(pool.Object, "metadata", "ownerReferences")
	unstructured.RemoveNestedField(pool.Object, "metadata", "finalizers")
	return pool
}

func waitForPoolNodesReady(ctx context.Context, poolName string, expected int, listReady func(context.Context) ([]string, error)) ([]string, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, nodeResourcePoolReadyTimeout)
	defer cancel()
	ticker := time.NewTicker(nodeResourcePoolPollInterval)
	defer ticker.Stop()
	for {
		names, err := listReady(timeoutCtx)
		switch {
		case err != nil:
			logrus.Warnf("Error checking readiness of NodeResource pool %s, will retry: %v", poolName, err)
		case len(names) >= expected:
			return names[:expected], nil
		default:
			logrus.Infof("Waiting for NodeResource pool %s: %d/%d node(s) Ready", poolName, len(names), expected)
		}
		select {
		case <-timeoutCtx.Done():
			return nil, fmt.Errorf("timed out after %s waiting for %d node(s) to become Ready", nodeResourcePoolReadyTimeout, expected)
		case <-ticker.C:
		}
	}
}

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
			continue
		}
		node, err := kubeClient.CoreV1().Nodes().Get(ctx, m.Status.NodeRef.Name, metav1.GetOptions{})
		if err != nil || !isNodeReady(node) {
			continue
		}
		ready = append(ready, node.Name)
	}
	sort.Strings(ready)
	return ready, nil
}

func readyNodeNamesForCAPIMachineSet(ctx context.Context, dc dynamic.Interface, kubeClient kubernetes.Interface, machineSetName string) ([]string, error) {
	list, err := dc.Resource(capiMachineGVR).Namespace(capiNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", capiMachineSetLabel, machineSetName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list CAPI machines for machineset %s: %w", machineSetName, err)
	}
	var ready []string
	for i := range list.Items {
		nodeName, found, _ := unstructured.NestedString(list.Items[i].Object, "status", "nodeRef", "name")
		if !found || nodeName == "" {
			continue
		}
		node, err := kubeClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil || !isNodeReady(node) {
			continue
		}
		ready = append(ready, node.Name)
	}
	sort.Strings(ready)
	return ready, nil
}

// teardownNodeResourcePool deletes a cloned pool MachineSet and waits for its machines to disappear.
func teardownNodeResourcePool(ctx context.Context, restConfig *rest.Config, pool *nodeResourcePool) {
	if pool == nil || pool.machineSetName == "" {
		return
	}
	logrus.Infof("Deleting NodeResource pool MachineSet %s (%d node(s))", pool.machineSetName, len(pool.nodeNames))

	var countMachines func(context.Context) (int, error)
	if pool.capi {
		dc, err := dynamic.NewForConfig(restConfig)
		if err != nil {
			logrus.Errorf("Failed to create dynamic client to tear down NodeResource pool %s: %v", pool.machineSetName, err)
			return
		}
		if err := dc.Resource(capiMachineSetGVR).Namespace(capiNamespace).Delete(ctx, pool.machineSetName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			logrus.Errorf("Failed to delete NodeResource pool MachineSet %s: %v", pool.machineSetName, err)
			return
		}
		countMachines = func(ctx context.Context) (int, error) {
			list, err := dc.Resource(capiMachineGVR).Namespace(capiNamespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", capiMachineSetLabel, pool.machineSetName),
			})
			if err != nil {
				return 0, err
			}
			return len(list.Items), nil
		}
	} else {
		machineClient, err := machineclient.NewForConfig(restConfig)
		if err != nil {
			logrus.Errorf("Failed to create machine API client to tear down NodeResource pool %s: %v", pool.machineSetName, err)
			return
		}
		if err := machineClient.MachineV1beta1().MachineSets(machineAPINamespace).Delete(ctx, pool.machineSetName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			logrus.Errorf("Failed to delete NodeResource pool MachineSet %s: %v", pool.machineSetName, err)
			return
		}
		countMachines = func(ctx context.Context) (int, error) {
			machines, err := machineClient.MachineV1beta1().Machines(machineAPINamespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", machineSetOwningLabel, pool.machineSetName),
			})
			if err != nil {
				return 0, err
			}
			return len(machines.Items), nil
		}
	}

	teardownCtx, cancel := context.WithTimeout(ctx, nodeResourcePoolTeardownTimeout)
	defer cancel()
	ticker := time.NewTicker(nodeResourcePoolPollInterval)
	defer ticker.Stop()
	for {
		n, listErr := countMachines(teardownCtx)
		if listErr != nil {
			logrus.Warnf("Error listing machines during teardown of %s: %v", pool.machineSetName, listErr)
		} else if n == 0 {
			logrus.Infof("NodeResource pool %s fully torn down", pool.machineSetName)
			return
		} else {
			logrus.Infof("Teardown: %d machine(s) still deleting for %s", n, pool.machineSetName)
		}
		select {
		case <-teardownCtx.Done():
			logrus.Warnf("Timed out waiting for machines of %s to be removed; cluster will GC them", pool.machineSetName)
			return
		case <-ticker.C:
		}
	}
}

var (
	capiMachineSetGVR = schema.GroupVersionResource{Group: "cluster.x-k8s.io", Version: "v1beta2", Resource: "machinesets"}
	capiMachineGVR    = schema.GroupVersionResource{Group: "cluster.x-k8s.io", Version: "v1beta2", Resource: "machines"}
)
