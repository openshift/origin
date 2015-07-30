package util

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	. "github.com/GoogleCloudPlatform/kubernetes/test/e2e"
	. "github.com/onsi/ginkgo"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/admin/policy"
	"github.com/openshift/origin/pkg/cmd/cli"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	testutil "github.com/openshift/origin/test/util"
	"github.com/spf13/cobra"
)

// CLI provides function to call the OpenShift CLI and Kubernetes and OpenShift
// REST clients.
type CLI struct {
	verb          string
	configPath    string
	globalArgs    []string
	commandArgs   []string
	finalArgs     []string
	stdout        io.Writer
	verbose       bool
	cmd           *cobra.Command
	kubeFramework *Framework
}

// NewCLI initialize the upstream E2E framework and set the namespace to match
// with the project name. Note that this function does not initialize the project
// role bindings for the namespace.
func NewCLI(project, configPath string) *CLI {
	client := &CLI{}
	client.kubeFramework = NewFramework(project)
	if len(configPath) == 0 {
		FatalErr(fmt.Errorf("The configPath can't be empty"))
	}
	client.configPath = configPath
	return client.SetNamespace(project)
}

// NewGinkoCLI initialize the upstream E2E framework and setup project role bindings.
// This function also initialize the Ginko BeforeEach hook to setup the role bindings
// for the namespace created by the E2E framework.
func NewGinkoCLI(project, configPath string) *CLI {
	// The namespace will be set by E2E framework.
	client := NewCLI(project, configPath).SetNamespace("")
	BeforeEach(func() {
		if len(client.Namespace()) > 0 {
			Logf("Adding project role bindings to %q namespace", client.Namespace())
			client.SetupRoleBindings()
		} else {
			Failf("Framework does not have the namespace set")
		}
	})
	return client
}

// OsFramework returns OpenShift framework
func (c *CLI) OsFramework() *OsFramework {
	return NewOsFramework(c.kubeFramework.Namespace, c.AdminRESTClient())
}

// KubeFramework returns Kubernetes framework
func (c *CLI) KubeFramework() *Framework {
	return c.kubeFramework
}

// SetNamespace overrides the namespace set by framework
func (c *CLI) SetNamespace(ns string) *CLI {
	if c.kubeFramework == nil {
		FatalErr(fmt.Errorf("The E2E framework must be initialized"))
	}
	c.kubeFramework.Namespace = &kapi.Namespace{ObjectMeta: kapi.ObjectMeta{Name: ns}}
	return c
}

// SetupRoleBindings setups a project role binding for the current namespace
func (c *CLI) SetupRoleBindings() *CLI {
	for _, binding := range bootstrappolicy.GetBootstrapServiceAccountProjectRoleBindings(c.Namespace()) {
		addRole := &policy.RoleModificationOptions{
			RoleName:            binding.RoleRef.Name,
			RoleNamespace:       binding.RoleRef.Namespace,
			RoleBindingAccessor: policy.NewLocalRoleBindingAccessor(c.Namespace(), c.AdminRESTClient()),
			Users:               binding.Users.List(),
			Groups:              binding.Groups.List(),
		}
		if err := addRole.AddRole(); err != nil {
			Failf("Unable to add role binding %+v to namespace %q: %v", addRole, c.Namespace(), err)
		}
	}
	return c
}

// Verbose turns on verbose messages for current command
func (c *CLI) Verbose() *CLI {
	c.verbose = true
	return c
}

// AdminRESTClient return the current project namespace REST client
func (c *CLI) AdminRESTClient() *client.Client {
	client, err := testutil.GetClusterAdminClient(c.configPath)
	if err != nil {
		FatalErr(err)
	}
	return client
}

// AdminKubeRESTClient returns the current project namespace Kubernetes client
func (c *CLI) AdminKubeRESTClient() *kclient.Client {
	if c.kubeFramework.Client != nil {
		return c.kubeFramework.Client
	}
	client, err := testutil.GetClusterAdminKubeClient(c.configPath)
	if err != nil {
		FatalErr(err)
	}
	return client
}

// Namespace returns the name of the namespace used in the current test case
func (c *CLI) Namespace() string {
	if c.kubeFramework == nil || c.kubeFramework.Namespace == nil {
		return ""
	}
	return c.kubeFramework.Namespace.Name
}

// SetOutput sets the default output for the command
func (c *CLI) SetOutput(out io.Writer) *CLI {
	c.stdout = out
	for _, subCmd := range c.cmd.Commands() {
		subCmd.SetOutput(c.stdout)
	}
	c.cmd.SetOutput(c.stdout)
	return c
}

// Run executes given OpenShift CLI command verb (iow. "oc <verb>").
// This function also override the default 'stdout' to redirect all output
// to a buffer and prepare the global flags such as namespace and config path.
func (c *CLI) Run(verb string) *CLI {
	out := new(bytes.Buffer)
	nc := &CLI{
		verb:          verb,
		kubeFramework: c.KubeFramework(),
		cmd:           cli.NewCommandCLI("oc", "openshift", out),
		globalArgs: []string{
			verb,
			fmt.Sprintf("--namespace=%s", c.Namespace()),
			fmt.Sprintf("--config=%s", c.configPath),
		},
	}
	return nc.SetOutput(out)
}

// Template sets a Go template for the OpenShift CLI command.
// This is equivalent of running "oc get foo -o template -t '{{ .spec }}'"
func (c *CLI) Template(t string) *CLI {
	if c.verb != "get" {
		FatalErr("Cannot use Template() for non-get verbs.")
	}
	templateArgs := []string{"--output=template", fmt.Sprintf("--template=%s", t)}
	commandArgs := append(c.commandArgs, templateArgs...)
	c.finalArgs = append(c.globalArgs, commandArgs...)
	c.cmd.SetArgs(c.finalArgs)
	return c
}

// Args sets the additional arguments for the OpenShift CLI command
func (c *CLI) Args(args ...string) *CLI {
	c.commandArgs = args
	c.finalArgs = append(c.globalArgs, c.commandArgs...)
	c.cmd.SetArgs(c.finalArgs)
	return c
}

func (c *CLI) printCmd() string {
	return strings.Join(c.finalArgs, " ")
}

// Output executes the command and return the output as string
func (c *CLI) Output() (string, error) {
	if c.verbose {
		fmt.Printf("DEBUG: oc %s\n", c.printCmd())
	}
	err := c.cmd.Execute()
	if err != nil {
		FatalErr(err)
	}
	out := c.stdout.(*bytes.Buffer)

	return strings.TrimSpace(out.String()), err
}

// Execute executes the current command and return error if the execution failed
// This function will set the default output to stdout.
func (c *CLI) Execute() error {
	out, err := c.Output()
	if _, err := io.Copy(os.Stdout, strings.NewReader(out+"\n")); err != nil {
		fmt.Printf("ERROR: Unable to copy the output to stdout")
	}
	os.Stdout.Sync()
	return err
}

// FatalErr exits the test in case a fatal error has occured.
func FatalErr(msg interface{}) {
	fmt.Printf("ERROR: %v\n", msg)
	os.Exit(1)
}
