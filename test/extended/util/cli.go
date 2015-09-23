package util

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli"
	cmdapi "github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	testutil "github.com/openshift/origin/test/util"
	"github.com/spf13/cobra"
	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client"
	clientcmd "k8s.io/kubernetes/pkg/client/clientcmd"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/test/e2e"
)

// CLI provides function to call the OpenShift CLI and Kubernetes and OpenShift
// REST clients.
type CLI struct {
	verb            string
	configPath      string
	adminConfigPath string
	username        string
	outputDir       string
	globalArgs      []string
	commandArgs     []string
	finalArgs       []string
	stdout          io.Writer
	verbose         bool
	cmd             *cobra.Command
	kubeFramework   *e2e.Framework
}

// NewCLI initialize the upstream E2E framework and set the namespace to match
// with the project name. Note that this function does not initialize the project
// role bindings for the namespace.
func NewCLI(project, adminConfigPath string) *CLI {
	client := &CLI{}
	client.kubeFramework = e2e.InitializeFramework(project, client.SetupProject)
	client.outputDir = os.TempDir()
	client.username = "admin"
	if len(adminConfigPath) == 0 {
		FatalErr(fmt.Errorf("You must set the KUBECONFIG variable to admin kubeconfig."))
	}
	client.adminConfigPath = adminConfigPath
	kcmdutil.BehaviorOnFatal(func(msg string) { panic(msg) })
	return client
}

// KubeFramework returns Kubernetes framework which contains helper functions
// specific for Kubernetes resources
func (c *CLI) KubeFramework() *e2e.Framework {
	return c.kubeFramework
}

// Username returns the name of currently logged user. If there is no user assigned
// for the current session, it returns 'admin'.
func (c *CLI) Username() string {
	return c.username
}

// ChangeUser changes the user used by the current CLI session.
func (c *CLI) ChangeUser(name string) *CLI {
	adminClientConfig, err := testutil.GetClusterAdminClientConfig(c.adminConfigPath)
	if err != nil {
		FatalErr(err)
	}
	_, _, clientConfig, err := testutil.GetClientForUser(*adminClientConfig, name)
	if err != nil {
		FatalErr(err)
	}

	kubeConfig, err := config.CreateConfig(c.Namespace(), clientConfig)
	if err != nil {
		FatalErr(err)
	}

	c.configPath = filepath.Join(c.outputDir, name+".kubeconfig")
	err = clientcmd.WriteToFile(*kubeConfig, c.configPath)
	if err != nil {
		FatalErr(err)
	}

	c.username = name
	fmt.Printf("INFO: configPath is now %q\n", c.configPath)
	return c
}

// SetNamespace sets a new namespace
func (c *CLI) SetNamespace(ns string) *CLI {
	c.kubeFramework.Namespace = &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: ns,
		},
	}
	return c
}

// SetOutputDir change the default output directory for temporary files
func (c *CLI) SetOutputDir(dir string) *CLI {
	c.outputDir = dir
	return c
}

// SetupProject creates a new project and assign a random user to the project.
// All resources will be then created within this project and Kubernetes E2E
// suite will destroy the project after test case finish.
// Note that the kubeClient is not used and serves just to make this function
// compatible with upstream function.
func (c *CLI) SetupProject(name string, kubeClient *kclient.Client) (*kapi.Namespace, error) {
	newNamespace := kapi.SimpleNameGenerator.GenerateName(fmt.Sprintf("extended-test-%s-", name))
	c.SetNamespace(newNamespace).ChangeUser(fmt.Sprintf("%s-user", c.Namespace()))
	e2e.Logf("The user is now %q", c.Username())

	projectOpts := cmdapi.NewProjectOptions{
		ProjectName: c.Namespace(),
		Client:      c.REST(),
		Out:         c.stdout,
	}
	e2e.Logf("Creating project %q", c.Namespace())
	return c.kubeFramework.Namespace, projectOpts.Run()
}

// Verbose turns on printing verbose messages when executing OpenShift commands
func (c *CLI) Verbose() *CLI {
	c.verbose = true
	return c
}

// REST provides an OpenShift REST client for the current user. If the user is not
// set, then it provides REST client for the cluster admin user
func (c *CLI) REST() *client.Client {
	_, clientConfig, err := configapi.GetKubeClient(c.configPath)
	osClient, err := client.New(clientConfig)
	if err != nil {
		FatalErr(err)
	}
	return osClient
}

// AdminREST provides an OpenShift REST client for the cluster admin user.
func (c *CLI) AdminREST() *client.Client {
	_, clientConfig, err := configapi.GetKubeClient(c.adminConfigPath)
	osClient, err := client.New(clientConfig)
	if err != nil {
		FatalErr(err)
	}
	return osClient
}

// KubeREST provides a Kubernetes REST client for the current namespace
func (c *CLI) KubeREST() *kclient.Client {
	kubeClient, _, err := configapi.GetKubeClient(c.configPath)
	if err != nil {
		FatalErr(err)
	}
	return kubeClient
}

// Namespace returns the name of the namespace used in the current test case.
// If the namespace is not set, an empty string is returned.
func (c *CLI) Namespace() string {
	if c.kubeFramework.Namespace == nil {
		return ""
	}
	return c.kubeFramework.Namespace.Name
}

// SetOutput allows to override the default command output
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
		verb:            verb,
		kubeFramework:   c.KubeFramework(),
		adminConfigPath: c.adminConfigPath,
		configPath:      c.configPath,
		username:        c.username,
		outputDir:       c.outputDir,
		cmd:             cli.NewCommandCLI("oc", "openshift", out),
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
func (c *CLI) Output() (out string, err error) {
	if c.verbose {
		fmt.Printf("DEBUG: oc %s\n", c.printCmd())
	}
	// Capture the panic and convert it to a regular error
	defer func() {
		if r := recover(); r != nil {
			out = fmt.Sprintf("%s", r)
			err = fmt.Errorf("PANIC: %s", out)
		}
	}()
	err = c.cmd.Execute()
	if c.verbose {
		fmt.Printf("DEBUG: %q\n", trimmedOutput(c.stdout))
	}
	return trimmedOutput(c.stdout), err
}

// Stdout returns the current stdout writer
func (c *CLI) Stdout() io.Writer {
	return c.stdout
}

// OutputToFile executes the command and store output to a file
func (c *CLI) OutputToFile(filename string) (string, error) {
	content, err := c.Output()
	if err != nil {
		return "", err
	}
	path := filepath.Join(c.outputDir, c.Namespace()+"-"+filename)
	return path, ioutil.WriteFile(path, []byte(content), 0644)
}

// Execute executes the current command and return error if the execution failed
// This function will set the default output to stdout.
func (c *CLI) Execute() error {
	out, err := c.Output()
	if err != nil {
		FatalErr(fmt.Errorf("%v", err))
	}
	if _, err := io.Copy(os.Stdout, strings.NewReader(out+"\n")); err != nil {
		fmt.Printf("ERROR: Unable to copy the output to stdout")
	}
	os.Stdout.Sync()
	return err
}

// trimmedOutput converts the stdout to a string and trims the trailing whitespaces
func trimmedOutput(stdout io.Writer) string {
	output, ok := stdout.(*bytes.Buffer)
	if !ok {
		fmt.Printf("WARNING: Unable to convert output to a buffer\n")
		return ""
	}
	return strings.TrimSpace(output.String())
}

// FatalErr exits the test in case a fatal error has occured.
func FatalErr(msg interface{}) {
	e2e.Failf("%v", msg)
}
