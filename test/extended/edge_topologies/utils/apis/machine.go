// Package apis provides Machine API utilities for machine.openshift.io (phase, nodeRef).
package apis

import (
	"context"
	"fmt"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/yaml"
)

// MachineGVR is the GroupVersionResource for Machine (machine.openshift.io/v1beta1). Use for API-based get/delete/patch.
var MachineGVR = schema.GroupVersionResource{
	Group: "machine.openshift.io", Version: "v1beta1", Resource: "machines",
}

// MachineStatus holds phase and nodeRef name from a Machine's status.
type MachineStatus struct {
	Phase    string
	NodeRef  string
	NotFound bool
}

// GetMachineStatus returns the Machine's status.phase and status.nodeRef.name using the dynamic client.
// Use this instead of oc get to interact with the cluster via API.
func GetMachineStatus(oc *exutil.CLI, machineName, namespace string) (MachineStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	dyn, err := dynamic.NewForConfig(oc.AdminConfig())
	if err != nil {
		return MachineStatus{}, fmt.Errorf("create dynamic client: %w", err)
	}
	u, err := dyn.Resource(MachineGVR).Namespace(namespace).Get(ctx, machineName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return MachineStatus{NotFound: true}, nil
		}
		return MachineStatus{}, err
	}
	status, _, _ := unstructured.NestedMap(u.Object, "status")
	phase, _, _ := unstructured.NestedString(status, "phase")
	nodeRef, _, _ := unstructured.NestedMap(status, "nodeRef")
	nodeRefName, _, _ := unstructured.NestedString(nodeRef, "name")
	return MachineStatus{Phase: phase, NodeRef: nodeRefName}, nil
}

// MachineExists returns true if the Machine exists in the namespace.
func MachineExists(oc *exutil.CLI, machineName, namespace string) (bool, error) {
	st, err := GetMachineStatus(oc, machineName, namespace)
	if err != nil {
		return false, err
	}
	return !st.NotFound, nil
}

// GetMachineProviderID returns the Machine's spec.providerID (e.g. baremetalhost:///...).
func GetMachineProviderID(oc *exutil.CLI, machineName, namespace string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	dyn, err := dynamic.NewForConfig(oc.AdminConfig())
	if err != nil {
		return "", fmt.Errorf("create dynamic client: %w", err)
	}
	u, err := dyn.Resource(MachineGVR).Namespace(namespace).Get(ctx, machineName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", fmt.Errorf("machine %s not found", machineName)
		}
		return "", err
	}
	id, ok, _ := unstructured.NestedString(u.Object, "spec", "providerID")
	if !ok || id == "" {
		return "", fmt.Errorf("machine %s has no spec.providerID", machineName)
	}
	return id, nil
}

// GetMachineYAML returns the Machine resource as YAML bytes (for backup). Uses the cluster API.
func GetMachineYAML(oc *exutil.CLI, machineName, namespace string) ([]byte, error) {
	ctx := context.Background()
	dyn, err := dynamic.NewForConfig(oc.AdminConfig())
	if err != nil {
		return nil, fmt.Errorf("create dynamic client: %w", err)
	}
	u, err := dyn.Resource(MachineGVR).Namespace(namespace).Get(ctx, machineName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	// Ensure apiVersion and kind for valid YAML
	content := u.UnstructuredContent()
	if content["apiVersion"] == nil {
		content["apiVersion"] = MachineGVR.GroupVersion().String()
	}
	if content["kind"] == nil {
		content["kind"] = "Machine"
	}
	return yaml.Marshal(content)
}
