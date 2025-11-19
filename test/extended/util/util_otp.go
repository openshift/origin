package util

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"k8s.io/apiserver/pkg/storage/names"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	corev1 "k8s.io/api/core/v1"
	crdv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// This file contains functions that extend origin's CLI to be compatible with OTP.

// NotShowInfo disables showing info in CLI output
func (c *CLI) NotShowInfo() *CLI {
	c.showInfo = false
	return c
}

// SetShowInfo enables showing info in CLI output
func (c *CLI) SetShowInfo() *CLI {
	c.showInfo = true
	return c
}

// SetGuestKubeconf sets the guest cluster kubeconf file
func (c *CLI) SetGuestKubeconf(guestKubeconf string) *CLI {
	c.guestConfigPath = guestKubeconf
	return c
}

// GetGuestKubeconf gets the guest cluster kubeconf file
func (c *CLI) GetGuestKubeconf() string {
	return c.guestConfigPath
}

// GuestKubeClient provides a Kubernetes client for the guest cluster user.
func (c *CLI) GuestKubeClient() kubernetes.Interface {
	return kubernetes.NewForConfigOrDie(c.GuestConfig())
}

// GuestConfig provides a REST client config for the guest cluster user.
func (c *CLI) GuestConfig() *rest.Config {
	clientConfig, err := GetClientConfig(c.guestConfigPath)
	if err != nil {
		FatalErr(err)
	}
	return turnOffRateLimiting(clientConfig)
}

// WithoutKubeconf simulates running commands without kubeconfig - OTP compatibility
// This is a no-op in origin but needed for OTP compatibility
func (c *CLI) WithoutKubeconf() *CLI {
	c.configPath = ""
	return c
}

// CreateNamespaceUDN creates a new namespace with required user defined network label during creation time only
// required for testing networking UDN features on 4.17z+
func (c *CLI) CreateNamespaceUDN() {
	// Create new namespace name here, because c.Namespace() will get existing test namespace
	// namespace Create func will be failed with below error
	// namespaces "e2e-test-xxx" already exists
	newNsName := names.SimpleNameGenerator.GenerateName(fmt.Sprintf("e2e-test-udn-%s-", c.kubeFramework.BaseName))
	c.SetNamespace(newNsName)

	labels := map[string]string{
		"k8s.ovn.org/primary-user-defined-network": "",
	}
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   newNsName,
			Labels: labels,
		},
	}
	_, err := c.AdminKubeClient().CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
	if err != nil {
		FatalErr(err)
	}
	c.kubeFramework.AddNamespacesToDelete(namespace)
}

// CreateSpecificNamespaceUDN creates a specific namespace with required user defined network label during creation time only
// required for testing networking UDN features on 4.17z+
// Important Note:  the namespace created by this function will not be automatically deleted, user need to explicitly delete the namespace after test is done
func (c *CLI) CreateSpecificNamespaceUDN(ns string) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
			Labels: map[string]string{
				"k8s.ovn.org/primary-user-defined-network": "udn-net",
			},
		},
	}
	_, err := c.AdminKubeClient().CoreV1().Namespaces().Create(context.Background(), namespace, metav1.CreateOptions{})
	if err != nil {
		FatalErr(err)
	}
}

// CreateSpecifiedNamespaceAsAdmin creates specified name namespace.
func (c *CLI) CreateSpecifiedNamespaceAsAdmin(namespace string) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	_, err := c.AdminKubeClient().CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	if err != nil {
		FatalErr(err)
	}
}

// DeleteSpecifiedNamespaceAsAdmin deletes specified name namespace.
func (c *CLI) DeleteSpecifiedNamespaceAsAdmin(namespace string) {
	err := c.AdminKubeClient().CoreV1().Namespaces().Delete(context.Background(), namespace, metav1.DeleteOptions{})
	if err != nil {
		// Log but don't fail if namespace doesn't exist
		fmt.Printf("Failed to delete namespace %s: %v\n", namespace, err)
	}
}

// AddPathsToDelete adds paths to be deleted after the test
func (c *CLI) AddPathsToDelete(dir string) {
	// OTP tracks paths to delete in a field, origin doesn't
	// For now, this is a no-op
}

// GetClientConfigForExtOIDCUser gets a client config for an external OIDC cluster
func (c *CLI) GetClientConfigForExtOIDCUser(tokenCacheDir string) *rest.Config {
	// This is a simplified implementation
	// In OTP this does more complex token caching
	return c.UserConfig()
}

// SilentOutput executes the command and returns stdout/stderr combined into one string
func (c *CLI) SilentOutput() (string, error) {
	// Save current verbose state
	wasVerbose := c.verbose
	c.verbose = false
	defer func() { c.verbose = wasVerbose }()

	return c.Output()
}

// AdminAPIExtensionsV1Client returns the API extensions v1 client
func (c *CLI) AdminAPIExtensionsV1Client() crdv1.ApiextensionsV1Interface {
	return crdv1.NewForConfigOrDie(c.AdminConfig())
}

// WithKubectl instructs the command should be invoked with binary kubectl, not oc.
func (c *CLI) WithKubectl() *CLI {
	c.execPath = "kubectl"
	return c
}

// OutputsToFiles executes the command and store the stdout in one file and stderr in another one
// The stdout output will be written to fileName+'.stdout'
// The stderr output will be written to fileName+'.stderr'
func (c *CLI) OutputsToFiles(fileName string) (string, string, error) {
	stdoutFilename := fileName + ".stdout"
	stderrFilename := fileName + ".stderr"

	stdout, stderr, err := c.Outputs()
	if err != nil {
		return "", "", err
	}
	stdoutPath := filepath.Join(e2e.TestContext.OutputDir, c.Namespace()+"-"+stdoutFilename)
	stderrPath := filepath.Join(e2e.TestContext.OutputDir, c.Namespace()+"-"+stderrFilename)

	if err := os.WriteFile(stdoutPath, []byte(stdout), 0644); err != nil {
		return "", "", err
	}

	if err := os.WriteFile(stderrPath, []byte(stderr), 0644); err != nil {
		return stdoutPath, "", err
	}

	return stdoutPath, stderrPath, nil
}

// Template sets a Go template for the OpenShift CLI command.
// This is equivalent of running "oc get foo -o template --template='{{ .spec }}'"
func (c *CLI) Template(t string) *CLI {
	if c.verb != "get" {
		FatalErr("Cannot use Template() for non-get verbs.")
	}
	templateArgs := []string{"--output=template", fmt.Sprintf("--template=%s", t)}
	c.commandArgs = append(c.commandArgs, templateArgs...)
	return c
}

// BackgroundRC executes the command in the background and returns the Cmd
// object which may be killed later via cmd.Process.Kill().  It returns a
// ReadCloser for stdout.  If in doubt, use Background().  Consult the os/exec
// documentation.
func (c *CLI) BackgroundRC() (*exec.Cmd, io.ReadCloser, error) {
	if c.verbose {
		fmt.Printf("DEBUG: oc %s\n", c.printCmd())
	}
	c.finalArgs = append(c.globalArgs, c.commandArgs...)
	cmd := exec.Command(c.execPath, c.finalArgs...)
	cmd.Stdin = c.stdin
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	e2e.Logf("Running '%s %s'", c.execPath, strings.Join(c.finalArgs, " "))

	err = cmd.Start()
	return cmd, stdout, err
}

func (c *CLI) GetKubeconf() string {
	return c.configPath
}

// SetKubeconf instructs the cluster kubeconf file is set
func (c *CLI) SetKubeconf(kubeconf string) *CLI {
	c.configPath = kubeconf
	return c
}

// SetAdminKubeconf instructs the admin cluster kubeconf file is set
func (c *CLI) SetAdminKubeconf(adminKubeconf string) *CLI {
	c.adminConfigPath = adminKubeconf
	return c
}

// AsGuestKubeconf returns a CLI configured to use the guest kubeconfig
func (c *CLI) AsGuestKubeconf() *CLI {
	// Create a copy of the CLI with guest kubeconfig enabled
	copy := *c
	// In OTP this sets a flag and uses guestConfigPath
	// We'll use the guestConfigPath as the configPath
	if c.guestConfigPath != "" {
		copy.configPath = c.guestConfigPath
	}
	return &copy
}
