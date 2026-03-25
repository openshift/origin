package compat_otp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"

	exutil "github.com/openshift/origin/test/extended/util"
)

// NewCLI initialize the upstream E2E framework and set the namespace to match
// with the project name. Note that this function does not initialize the project
// role bindings for the namespace.
func NewCLI(project, adminConfigPath string) *exutil.CLI {
	return exutil.NewCLI(project)
}

// NewCLIWithoutNamespace initializes the upstream E2E framework in a
// non-namespaced context. Most commonly used for global object (non-namespaced) tests.
func NewCLIWithoutNamespace(project string) *exutil.CLI {
	cli := exutil.NewCLI(project)
	return cli.WithoutNamespace()
}

// NewCLIForKube initializes the exutil.CLI for tests against pure Kubernetes environments
func NewCLIForKube(basename string) *exutil.CLI {
	return exutil.NewCLI(basename)
}

// NewCLIForKubeOpenShift provides a exutil.CLI that detects Kubernetes or OpenShift
func NewCLIForKubeOpenShift(basename string) *exutil.CLI {
	// Check if it's a Kubernetes-only environment
	if os.Getenv(EnvIsKubernetesCluster) == "true" {
		return NewCLIForKube(basename)
	}

	// Check if it's HyperShift environment with Kubernetes management cluster
	// In this case, we use exutil.NewCLIWithoutNamespace to avoid OpenShift-specific
	// initialization (like clusterversion checks) that would fail on pure Kubernetes
	if isHyperShiftWithKubernetesManagement() {
		return exutil.NewCLIWithoutNamespace(basename)
	}

	return exutil.NewCLI(basename)
}

// determineExecCLI returns the appropriate CLI object based on guest kubeconfig availability.
// If guest kubeconfig is set, returns CLI with guest config; otherwise returns CLI with admin config.
// This ensures operations target the correct cluster (management vs. guest) automatically.
func determineExecCLI(oc *exutil.CLI) *exutil.CLI {
	if oc.GetGuestKubeconf() != "" {
		return oc.AsGuestKubeconf()
	}
	return oc.AsAdmin()
}

// IsNamespacePrivileged checks if a namespace has privileged SCC
func IsNamespacePrivileged(oc *exutil.CLI, namespace string) (bool, error) {
	// Check for the Kubernetes Pod Security Admission 'enforce: privileged' label.
	// This is the direct confirmation that the namespace's admission controller
	// will allow an unrestricted pod (like the one created by 'oc debug node').
	stdout, err := determineExecCLI(oc).Run("get").Args("ns", namespace, "-o", `jsonpath={.metadata.labels.pod-security\.kubernetes\.io/enforce}`).Output()

	if err != nil {
		return false, err
	}

	// The namespace is privileged if it explicitly enforces the privileged standard.
	if labelValue := strings.TrimSpace(stdout); labelValue == "privileged" {
		return true, nil
	}

	return false, nil
}

// SetNamespacePrivileged sets a namespace to use privileged SCC
func SetNamespacePrivileged(oc *exutil.CLI, namespace string) error {
	err := determineExecCLI(oc).Run("label").Args("ns", namespace, "pod-security.kubernetes.io/enforce=privileged", "pod-security.kubernetes.io/audit=privileged", "pod-security.kubernetes.io/warn=privileged", "security.openshift.io/scc.podSecurityLabelSync=false", "--overwrite").Execute()
	if err != nil {
		return fmt.Errorf("failed to set namespace %s privileged: %v", namespace, err)
	}
	return nil
}

// RecoverNamespaceRestricted recovers a namespace to restricted mode
func RecoverNamespaceRestricted(oc *exutil.CLI, namespace string) error {
	err := determineExecCLI(oc).Run("label").Args("ns", namespace, "pod-security.kubernetes.io/enforce-", "pod-security.kubernetes.io/audit-", "pod-security.kubernetes.io/warn-", "security.openshift.io/scc.podSecurityLabelSync-").Execute()
	if err != nil {
		return fmt.Errorf("failed to recover namespace %s to restricted: %v", namespace, err)
	}
	return nil
}

// Keep other utility functions that don't depend on the exutil.CLI struct...

// FatalErr prints the error and exits the process
func FatalErr(msg interface{}) {
	// Allow errors to include more information, like stack traces when error has a
	// format for Sprintf to use.
	if err, ok := msg.(error); ok {
		errorMsg := fmt.Sprintf("%+v", err)
		// When the error includes a stack trace, make the output more readable
		if stackTrace := strings.Split(errorMsg, "\n"); len(stackTrace) > 1 {
			g.GinkgoWriter.Printf("FATAL ERROR: %s\n\n", stackTrace[0])
			for _, s := range stackTrace[1:] {
				g.GinkgoWriter.Println(s)
			}
		} else {
			g.Fail(fmt.Sprintf("FATAL ERROR: %v", err))
		}
	} else {
		g.Fail(fmt.Sprintf("FATAL ERROR: %v", msg))
	}
}

// GetPodLogs retrieves logs from a pod
func GetPodLogs(oc *exutil.CLI, pod, container, since string) (string, error) {
	args := []string{"logs", pod}
	if container != "" {
		args = append(args, "-c", container)
	}
	if since != "" {
		args = append(args, "--since", since)
	}
	return oc.Run("get").Args(args...).Output()
}

// isHyperShiftWithKubernetesManagement checks if this is a HyperShift environment
// where the current KUBECONFIG points to a Kubernetes management cluster (not OpenShift).
// This is typical for HyperShift on AKS, EKS, or GKE where the management cluster
// is a pure Kubernetes cluster.
func isHyperShiftWithKubernetesManagement() bool {
	// Check 1: Is HYPERSHIFT environment variable set?
	if os.Getenv("HYPERSHIFT") != "true" {
		return false
	}

	// Check 2: Does the cluster have clusterversion API?
	// If it doesn't exist, it's likely a Kubernetes cluster (not OpenShift)
	return !hasClusterVersionAPI()
}

// hasClusterVersionAPI checks if the cluster has the OpenShift clusterversion API.
// Returns true if the API exists (OpenShift cluster), false otherwise (Kubernetes cluster).
func hasClusterVersionAPI() bool {
	// Create context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to list API resources in the config.openshift.io group
	cmd := exec.CommandContext(ctx, "oc", "api-resources", "--api-group=config.openshift.io", "-o=name")

	// Use KUBECONFIG from environment if set
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check for critical errors that should not be silently ignored
		if errors.Is(err, exec.ErrNotFound) {
			// oc binary not found - this is a real failure but treat as no API
			// since we can't determine the cluster type
			return false
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			// Command timed out - treat as no API available
			return false
		}

		// For other errors, check if it's an expected "API not found" pattern
		outputStr := string(output)
		// When the API group doesn't exist, oc returns error with no output or specific messages
		if outputStr == "" ||
		   strings.Contains(outputStr, "the server doesn't have a resource type") ||
		   strings.Contains(outputStr, "no resources found") {
			return false
		}

		// Unexpected error - log it but return false to be safe
		// We can't determine definitively, so assume no OpenShift API
		return false
	}

	// Check if clusterversions resource exists in the output
	return strings.Contains(string(output), "clusterversions")
}
