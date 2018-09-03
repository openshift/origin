package util

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"

	authorizationapiv1 "k8s.io/api/authorization/v1"
	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/discovery/cached"
	kclientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	kinternalclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	appsv1client "github.com/openshift/client-go/apps/clientset/versioned"
	buildv1client "github.com/openshift/client-go/build/clientset/versioned"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned"
	templateclient "github.com/openshift/client-go/template/clientset/versioned"
	_ "github.com/openshift/origin/pkg/api/install"
	authorizationclientset "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	imageclientset "github.com/openshift/origin/pkg/image/generated/internalclientset"
	"github.com/openshift/origin/pkg/oc/lib/kubeconfig"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	projectclientset "github.com/openshift/origin/pkg/project/generated/internalclientset"
	routeclientset "github.com/openshift/origin/pkg/route/generated/internalclientset"
	securityclientset "github.com/openshift/origin/pkg/security/generated/internalclientset"
	templateclientset "github.com/openshift/origin/pkg/template/generated/internalclientset"
	userclientset "github.com/openshift/origin/pkg/user/generated/internalclientset"
	testutil "github.com/openshift/origin/test/util"
)

// CLI provides function to call the OpenShift CLI and Kubernetes and OpenShift
// clients.
type CLI struct {
	execPath           string
	verb               string
	configPath         string
	adminConfigPath    string
	username           string
	globalArgs         []string
	commandArgs        []string
	finalArgs          []string
	namespacesToDelete []string
	stdin              *bytes.Buffer
	stdout             io.Writer
	stderr             io.Writer
	verbose            bool
	withoutNamespace   bool
	kubeFramework      *e2e.Framework
}

// NewCLI initialize the upstream E2E framework and set the namespace to match
// with the project name. Note that this function does not initialize the project
// role bindings for the namespace.
func NewCLI(project, adminConfigPath string) *CLI {
	client := &CLI{}

	// must be registered before the e2e framework aftereach
	g.AfterEach(client.TeardownProject)

	client.kubeFramework = e2e.NewDefaultFramework(project)
	client.kubeFramework.SkipNamespaceCreation = true
	client.username = "admin"
	client.execPath = "oc"
	if len(adminConfigPath) == 0 {
		FatalErr(fmt.Errorf("you must set the KUBECONFIG variable to admin kubeconfig"))
	}
	client.adminConfigPath = adminConfigPath

	g.BeforeEach(client.SetupProject)

	return client
}

// ProwGCPSetup makes sure certain required env vars are available in the case
// that extended tests are invoked directly via calls to ginkgo/extended.test
func ProwGCPSetup(oc *CLI) {
	tn := os.Getenv("OS_TEST_NAMESPACE")
	ad := os.Getenv("ARTIFACT_DIR")
	btd := os.Getenv("BASETMPDIR")
	e2e.Logf("OS_TEST_NAMESPACE env setting %s, ARTIFACT_DIR env setting %s BASETMPDIR %s", tn, ad, btd)
	if len(strings.TrimSpace(tn)) == 0 {
		os.Setenv("OS_TEST_NAMESPACE", oc.Namespace())
		e2e.Logf("OS_TEST_NAMESPACE env setting now %s", os.Getenv("OS_TEST_NAMESPACE"))
	}
	if len(strings.TrimSpace(ad)) == 0 {
		os.Setenv("ARTIFACT_DIR", "/tmp/artifacts")
		e2e.Logf("ARTIFACT_DIR env setting now %s", os.Getenv("ARTIFACT_DIR"))
	}
	if len(strings.TrimSpace(btd)) == 0 {
		os.Setenv("BASETMPDIR", "/tmp/shared")
		e2e.Logf("BASETMPDIR setting is now %s", os.Getenv("BASETMPDIR"))
	}

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

// AsAdmin changes current config file path to the admin config.
func (c *CLI) AsAdmin() *CLI {
	nc := *c
	nc.configPath = c.adminConfigPath
	return &nc
}

// ChangeUser changes the user used by the current CLI session.
func (c *CLI) ChangeUser(name string) *CLI {
	adminClientConfig, err := testutil.GetClusterAdminClientConfig(c.adminConfigPath)
	if err != nil {
		FatalErr(err)
	}
	_, clientConfig, err := testutil.GetClientForUser(adminClientConfig, name)
	if err != nil {
		FatalErr(err)
	}

	kubeConfig, err := kubeconfig.CreateConfig(c.Namespace(), clientConfig)
	if err != nil {
		FatalErr(err)
	}

	f, err := ioutil.TempFile("", "configfile")
	if err != nil {
		FatalErr(err)
	}
	c.configPath = f.Name()
	err = clientcmd.WriteToFile(*kubeConfig, c.configPath)
	if err != nil {
		FatalErr(err)
	}

	c.username = name
	e2e.Logf("configPath is now %q", c.configPath)
	return c
}

// SetNamespace sets a new namespace
func (c *CLI) SetNamespace(ns string) *CLI {
	c.kubeFramework.Namespace = &kapiv1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	return c
}

// WithoutNamespace instructs the command should be invoked without adding --namespace parameter
func (c CLI) WithoutNamespace() *CLI {
	c.withoutNamespace = true
	return &c
}

// SetupProject creates a new project and assign a random user to the project.
// All resources will be then created within this project.
func (c *CLI) SetupProject() {
	newNamespace := names.SimpleNameGenerator.GenerateName(fmt.Sprintf("e2e-test-%s-", c.kubeFramework.BaseName))
	c.SetNamespace(newNamespace).ChangeUser(fmt.Sprintf("%s-user", newNamespace))
	e2e.Logf("The user is now %q", c.Username())
	ProwGCPSetup(c)

	e2e.Logf("Creating project %q", newNamespace)
	_, err := c.ProjectClient().Project().ProjectRequests().Create(&projectapi.ProjectRequest{
		ObjectMeta: metav1.ObjectMeta{Name: newNamespace},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	// TODO: remove when https://github.com/kubernetes/kubernetes/pull/62606 merges and is in origin
	c.namespacesToDelete = append(c.namespacesToDelete, newNamespace)

	e2e.Logf("Waiting on permissions in project %q ...", newNamespace)
	err = WaitForSelfSAR(1*time.Second, 60*time.Second, c.KubeClient(), authorizationapiv1.SelfSubjectAccessReviewSpec{
		ResourceAttributes: &authorizationapiv1.ResourceAttributes{
			Namespace: newNamespace,
			Verb:      "create",
			Group:     "",
			Resource:  "pods",
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

// SetupProject creates a new project and assign a random user to the project.
// All resources will be then created within this project.
func (c *CLI) CreateProject() string {
	newNamespace := names.SimpleNameGenerator.GenerateName(fmt.Sprintf("e2e-test-%s-", c.kubeFramework.BaseName))
	e2e.Logf("Creating project %q", newNamespace)
	_, err := c.ProjectClient().Project().ProjectRequests().Create(&projectapi.ProjectRequest{
		ObjectMeta: metav1.ObjectMeta{Name: newNamespace},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	// TODO: remove when https://github.com/kubernetes/kubernetes/pull/62606 merges and is in origin
	c.namespacesToDelete = append(c.namespacesToDelete, newNamespace)

	e2e.Logf("Waiting on permissions in project %q ...", newNamespace)
	err = WaitForSelfSAR(1*time.Second, 60*time.Second, c.KubeClient(), authorizationapiv1.SelfSubjectAccessReviewSpec{
		ResourceAttributes: &authorizationapiv1.ResourceAttributes{
			Namespace: newNamespace,
			Verb:      "create",
			Group:     "",
			Resource:  "pods",
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	return newNamespace
}

// TeardownProject removes projects created by this test.
func (c *CLI) TeardownProject() {
	if g.CurrentGinkgoTestDescription().Failed && e2e.TestContext.DumpLogsOnFailure {
		e2e.DumpAllNamespaceInfo(c.kubeFramework.ClientSet, c.Namespace())
	}

	if len(c.configPath) > 0 {
		os.Remove(c.configPath)
	}
	if e2e.TestContext.DeleteNamespace && len(c.namespacesToDelete) > 0 {
		timeout := e2e.DefaultNamespaceDeletionTimeout
		if c.kubeFramework.NamespaceDeletionTimeout != 0 {
			timeout = c.kubeFramework.NamespaceDeletionTimeout
		}
		e2e.DeleteNamespaces(c.kubeFramework.ClientSet, c.namespacesToDelete, nil)
		e2e.WaitForNamespacesDeleted(c.kubeFramework.ClientSet, c.namespacesToDelete, timeout)
	}
}

// Verbose turns on printing verbose messages when executing OpenShift commands
func (c *CLI) Verbose() *CLI {
	c.verbose = true
	return c
}

func (c *CLI) RESTMapper() meta.RESTMapper {
	ret := restmapper.NewDeferredDiscoveryRESTMapper(cached.NewMemCacheClient(c.KubeClient().Discovery()))
	ret.Reset()
	return ret
}

func (c *CLI) AppsClient() appsv1client.Interface {
	client, err := appsv1client.NewForConfig(c.UserConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

func (c *CLI) AuthorizationClient() authorizationclientset.Interface {
	client, err := authorizationclientset.NewForConfig(c.UserConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

func (c *CLI) BuildClient() buildv1client.Interface {
	client, err := buildv1client.NewForConfig(c.UserConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

func (c *CLI) ImageClient() imageclientset.Interface {
	client, err := imageclientset.NewForConfig(c.UserConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

func (c *CLI) ProjectClient() projectclientset.Interface {
	client, err := projectclientset.NewForConfig(c.UserConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

func (c *CLI) RouteClient() routeclientset.Interface {
	client, err := routeclientset.NewForConfig(c.UserConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

// Client provides an OpenShift client for the current user. If the user is not
// set, then it provides client for the cluster admin user
func (c *CLI) InternalTemplateClient() templateclientset.Interface {
	client, err := templateclientset.NewForConfig(c.UserConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

// Client provides an OpenShift client for the current user. If the user is not
// set, then it provides client for the cluster admin user
func (c *CLI) TemplateClient() templateclient.Interface {
	client, err := templateclient.NewForConfig(c.UserConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

func (c *CLI) UserClient() userclientset.Interface {
	client, err := userclientset.NewForConfig(c.UserConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

func (c *CLI) AdminAppsClient() appsv1client.Interface {
	client, err := appsv1client.NewForConfig(c.AdminConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

func (c *CLI) AdminAuthorizationClient() authorizationclientset.Interface {
	client, err := authorizationclientset.NewForConfig(c.AdminConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

func (c *CLI) AdminBuildClient() buildv1client.Interface {
	client, err := buildv1client.NewForConfig(c.AdminConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

func (c *CLI) AdminImageClient() imagev1client.Interface {
	client, err := imagev1client.NewForConfig(c.AdminConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

// DEPRECATED: use external
func (c *CLI) AdminInternalImageClient() imageclientset.Interface {
	client, err := imageclientset.NewForConfig(c.AdminConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

func (c *CLI) AdminProjectClient() projectclientset.Interface {
	client, err := projectclientset.NewForConfig(c.AdminConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

func (c *CLI) AdminRouteClient() routeclientset.Interface {
	client, err := routeclientset.NewForConfig(c.AdminConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

// AdminClient provides an OpenShift client for the cluster admin user.
func (c *CLI) AdminInternalTemplateClient() templateclientset.Interface {
	client, err := templateclientset.NewForConfig(c.AdminConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

func (c *CLI) AdminTemplateClient() templateclient.Interface {
	client, err := templateclient.NewForConfig(c.AdminConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

func (c *CLI) AdminUserClient() userclientset.Interface {
	client, err := userclientset.NewForConfig(c.AdminConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

func (c *CLI) AdminSecurityClient() securityclientset.Interface {
	client, err := securityclientset.NewForConfig(c.AdminConfig())
	if err != nil {
		FatalErr(err)
	}
	return client
}

// KubeClient provides a Kubernetes client for the current namespace
func (c *CLI) KubeClient() kclientset.Interface {
	return kclientset.NewForConfigOrDie(c.UserConfig())
}

// KubeClient provides a Kubernetes client for the current namespace
func (c *CLI) InternalKubeClient() kinternalclientset.Interface {
	return kinternalclientset.NewForConfigOrDie(c.UserConfig())
}

// AdminKubeClient provides a Kubernetes client for the cluster admin user.
func (c *CLI) AdminKubeClient() kclientset.Interface {
	return kclientset.NewForConfigOrDie(c.AdminConfig())
}

// AdminKubeClient provides a Kubernetes client for the cluster admin user.
func (c *CLI) InternalAdminKubeClient() kinternalclientset.Interface {
	return kinternalclientset.NewForConfigOrDie(c.AdminConfig())
}

func (c *CLI) UserConfig() *restclient.Config {
	clientConfig, err := configapi.GetClientConfig(c.configPath, nil)
	if err != nil {
		FatalErr(err)
	}
	return clientConfig
}

func (c *CLI) AdminConfig() *restclient.Config {
	clientConfig, err := configapi.GetClientConfig(c.adminConfigPath, nil)
	if err != nil {
		FatalErr(err)
	}
	return clientConfig
}

// Namespace returns the name of the namespace used in the current test case.
// If the namespace is not set, an empty string is returned.
func (c *CLI) Namespace() string {
	if c.kubeFramework.Namespace == nil {
		return ""
	}
	return c.kubeFramework.Namespace.Name
}

// setOutput allows to override the default command output
func (c *CLI) setOutput(out io.Writer) *CLI {
	c.stdout = out
	return c
}

// Run executes given OpenShift CLI command verb (iow. "oc <verb>").
// This function also override the default 'stdout' to redirect all output
// to a buffer and prepare the global flags such as namespace and config path.
func (c *CLI) Run(commands ...string) *CLI {
	in, out, errout := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	nc := &CLI{
		execPath:        c.execPath,
		verb:            commands[0],
		kubeFramework:   c.KubeFramework(),
		adminConfigPath: c.adminConfigPath,
		configPath:      c.configPath,
		username:        c.username,
		globalArgs: append(commands, []string{
			fmt.Sprintf("--config=%s", c.configPath),
		}...),
	}
	if !c.withoutNamespace {
		nc.globalArgs = append(nc.globalArgs, fmt.Sprintf("--namespace=%s", c.Namespace()))
	}
	nc.stdin, nc.stdout, nc.stderr = in, out, errout
	return nc.setOutput(c.stdout)
}

// Template sets a Go template for the OpenShift CLI command.
// This is equivalent of running "oc get foo -o template --template='{{ .spec }}'"
func (c *CLI) Template(t string) *CLI {
	if c.verb != "get" {
		FatalErr("Cannot use Template() for non-get verbs.")
	}
	templateArgs := []string{"--output=template", fmt.Sprintf("--template=%s", t)}
	commandArgs := append(c.commandArgs, templateArgs...)
	c.finalArgs = append(c.globalArgs, commandArgs...)
	return c
}

// InputString adds expected input to the command
func (c *CLI) InputString(input string) *CLI {
	c.stdin.WriteString(input)
	return c
}

// Args sets the additional arguments for the OpenShift CLI command
func (c *CLI) Args(args ...string) *CLI {
	c.commandArgs = args
	c.finalArgs = append(c.globalArgs, c.commandArgs...)
	return c
}

func (c *CLI) printCmd() string {
	return strings.Join(c.finalArgs, " ")
}

type ExitError struct {
	Cmd    string
	StdErr string
	*exec.ExitError
}

// Output executes the command and returns stdout/stderr combined into one string
func (c *CLI) Output() (string, error) {
	if c.verbose {
		fmt.Printf("DEBUG: oc %s\n", c.printCmd())
	}
	cmd := exec.Command(c.execPath, c.finalArgs...)
	cmd.Stdin = c.stdin
	e2e.Logf("Running '%s %s'", c.execPath, strings.Join(c.finalArgs, " "))
	out, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(out))
	switch err.(type) {
	case nil:
		c.stdout = bytes.NewBuffer(out)
		return trimmed, nil
	case *exec.ExitError:
		e2e.Logf("Error running %v:\n%s", cmd, trimmed)
		return trimmed, &ExitError{ExitError: err.(*exec.ExitError), Cmd: c.execPath + " " + strings.Join(c.finalArgs, " "), StdErr: trimmed}
	default:
		FatalErr(fmt.Errorf("unable to execute %q: %v", c.execPath, err))
		// unreachable code
		return "", nil
	}
}

// Outputs executes the command and returns the stdout/stderr output as separate strings
func (c *CLI) Outputs() (string, string, error) {
	if c.verbose {
		fmt.Printf("DEBUG: oc %s\n", c.printCmd())
	}
	cmd := exec.Command(c.execPath, c.finalArgs...)
	cmd.Stdin = c.stdin
	e2e.Logf("Running '%s %s'", c.execPath, strings.Join(c.finalArgs, " "))
	//out, err := cmd.CombinedOutput()
	var stdErrBuff, stdOutBuff bytes.Buffer
	cmd.Stdout = &stdOutBuff
	cmd.Stderr = &stdErrBuff
	err := cmd.Run()

	stdOutBytes := stdOutBuff.Bytes()
	stdErrBytes := stdErrBuff.Bytes()
	stdOut := strings.TrimSpace(string(stdOutBytes))
	stdErr := strings.TrimSpace(string(stdErrBytes))
	switch err.(type) {
	case nil:
		c.stdout = bytes.NewBuffer(stdOutBytes)
		c.stderr = bytes.NewBuffer(stdErrBytes)
		return stdOut, stdErr, nil
	case *exec.ExitError:
		e2e.Logf("Error running %v:\nStdOut>\n%s\nStdErr>\n%s\n", cmd, stdOut, stdErr)
		return stdOut, stdErr, err
	default:
		FatalErr(fmt.Errorf("unable to execute %q: %v", c.execPath, err))
		// unreachable code
		return "", "", nil
	}
}

// Background executes the command in the background and returns the Cmd object
// which may be killed later via cmd.Process.Kill().  It also returns buffers
// holding the stdout & stderr of the command, which may be read from only after
// calling cmd.Wait().
func (c *CLI) Background() (*exec.Cmd, *bytes.Buffer, *bytes.Buffer, error) {
	if c.verbose {
		fmt.Printf("DEBUG: oc %s\n", c.printCmd())
	}
	cmd := exec.Command(c.execPath, c.finalArgs...)
	cmd.Stdin = c.stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout = bufio.NewWriter(&stdout)
	cmd.Stderr = bufio.NewWriter(&stderr)

	e2e.Logf("Running '%s %s'", c.execPath, strings.Join(c.finalArgs, " "))

	err := cmd.Start()
	return cmd, &stdout, &stderr, err
}

// BackgroundRC executes the command in the background and returns the Cmd
// object which may be killed later via cmd.Process.Kill().  It returns a
// ReadCloser for stdout.  If in doubt, use Background().  Consult the os/exec
// documentation.
func (c *CLI) BackgroundRC() (*exec.Cmd, io.ReadCloser, error) {
	if c.verbose {
		fmt.Printf("DEBUG: oc %s\n", c.printCmd())
	}
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

// OutputToFile executes the command and store output to a file
func (c *CLI) OutputToFile(filename string) (string, error) {
	content, err := c.Output()
	if err != nil {
		return "", err
	}
	path := filepath.Join(e2e.TestContext.OutputDir, c.Namespace()+"-"+filename)
	return path, ioutil.WriteFile(path, []byte(content), 0644)
}

// Execute executes the current command and return error if the execution failed
// This function will set the default output to Ginkgo writer.
func (c *CLI) Execute() error {
	out, err := c.Output()
	if _, err := io.Copy(g.GinkgoWriter, strings.NewReader(out+"\n")); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR: Unable to copy the output to ginkgo writer")
	}
	os.Stdout.Sync()
	return err
}

// FatalErr exits the test in case a fatal error has occurred.
func FatalErr(msg interface{}) {
	// the path that leads to this being called isn't always clear...
	fmt.Fprintln(g.GinkgoWriter, string(debug.Stack()))
	e2e.Failf("%v", msg)
}
