package compat_otp

import (
	"fmt"
	"os"
	"strings"

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
	return exutil.NewCLI(basename)
}

// IsNamespacePrivileged checks if a namespace has privileged SCC
func IsNamespacePrivileged(oc *exutil.CLI, namespace string) (bool, error) {
	// Check for the Kubernetes Pod Security Admission 'enforce: privileged' label.
	// This is the direct confirmation that the namespace's admission controller
	// will allow an unrestricted pod (like the one created by 'oc debug node').
	stdout, err := oc.AsAdmin().Run("get").Args("ns", namespace, "-o", `jsonpath={.metadata.labels.pod-security\.kubernetes\.io/enforce}`).Output()

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
	err := oc.AsAdmin().Run("label").Args("ns", namespace, "pod-security.kubernetes.io/enforce=privileged", "pod-security.kubernetes.io/audit=privileged", "pod-security.kubernetes.io/warn=privileged", "security.openshift.io/scc.podSecurityLabelSync=false", "--overwrite").Execute()
	if err != nil {
		return fmt.Errorf("failed to set namespace %s privileged: %v", namespace, err)
	}
	return nil
}

// RecoverNamespaceRestricted recovers a namespace to restricted mode
func RecoverNamespaceRestricted(oc *exutil.CLI, namespace string) error {
	err := oc.AsAdmin().Run("label").Args("ns", namespace, "pod-security.kubernetes.io/enforce-", "pod-security.kubernetes.io/audit-", "pod-security.kubernetes.io/warn-", "security.openshift.io/scc.podSecurityLabelSync-").Execute()
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
