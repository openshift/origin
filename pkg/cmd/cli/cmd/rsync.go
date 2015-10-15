package cmd

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/tar"
	"github.com/spf13/cobra"
	kvalidation "k8s.io/kubernetes/pkg/api/validation"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kubecmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	// RsyncRecommendedName is the recommended name for the rsync command
	RsyncRecommendedName = "rsync"

	rsyncLong = `
Copy local files to a container

This command will attempt to copy local files to a remote container. It will default to the
first container if none is specified, and will attempt to use 'rsync' if available locally and in the
container. If 'rsync' is not present, it will attempt to use 'tar' to send files to the container.`

	rsyncExample = `
  # Synchronize a local directory with a pod directory
  $ %[1]s ./local/dir/ POD:/remote/dir`

	defaultRsyncExecutable = "rsync"
	defaultTarExecutable   = "tar"
)

var (
	testRsyncCommand = []string{"rsync", "--version"}
	testTarCommand   = []string{"tar", "--version"}
)

// RsyncOptions holds the options to execute the sync command
type RsyncOptions struct {
	Namespace      string
	PodName        string
	ContainerName  string
	Source         string
	Destination    string
	DestinationDir string
	RshCommand     string
	UseTar         bool
	Quiet          bool
	Delete         bool

	Out    io.Writer
	ErrOut io.Writer

	LocalExecutor  executor
	RemoteExecutor executor
	Tar            tar.Tar
	PodClient      kclient.PodInterface
}

// executor executes commands
type executor interface {
	Execute(command []string, in io.Reader, out, err io.Writer) error
}

// NewCmdRsync creates a new sync command
func NewCmdRsync(name, parent string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	tarHelper := tar.New()
	tarHelper.SetExclusionPattern(nil)
	o := RsyncOptions{
		Out:           out,
		ErrOut:        errOut,
		LocalExecutor: &defaultLocalExecutor{},
		Tar:           tarHelper,
	}
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s SOURCE_DIR POD:DESTINATION_DIR", name),
		Short:   "Copy local files to a pod",
		Long:    rsyncLong,
		Example: fmt.Sprintf(rsyncExample, parent+" "+name),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, c, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.RunRsync())
		},
	}

	cmd.Flags().StringVarP(&o.ContainerName, "container", "c", "", "Container within the pod")
	cmd.Flags().BoolVarP(&o.Quiet, "quiet", "q", false, "Quiet copy")
	cmd.Flags().BoolVar(&o.Delete, "delete", false, "Delete files not present in source")
	cmd.Flags().BoolVar(&o.UseTar, "use-tar", false, "Use tar instead of rsync")
	return cmd
}

func parseDestination(destination string) (string, string, error) {
	parts := strings.SplitN(destination, ":", 2)
	if len(parts) < 2 || len(parts[0]) == 0 || len(parts[1]) == 0 {
		return "", "", fmt.Errorf("invalid destination %s: must be of the form PODNAME:DESTINATION_DIR", destination)
	}
	valid, msg := kvalidation.ValidatePodName(parts[0], false)
	if !valid {
		return "", "", fmt.Errorf("invalid pod name %s: %s", parts[0], msg)
	}
	return parts[0], parts[1], nil
}

// Complete verifies command line arguments and loads data from the command environment
func (o *RsyncOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	switch n := len(args); {
	case n == 0:
		cmd.Help()
		fallthrough
	case n < 2:
		return kcmdutil.UsageError(cmd, "SOURCE_DIR and POD:DESTINATION_DIR are required arguments")
	case n > 2:
		return kcmdutil.UsageError(cmd, "only SOURCE_DIR and POD:DESTINATION_DIR should be specified as arguments")
	}

	// Set main command arguments
	o.Source = args[0]
	o.Destination = args[1]

	// Determine pod name
	var err error
	o.PodName, o.DestinationDir, err = parseDestination(o.Destination)
	if err != nil {
		return kcmdutil.UsageError(cmd, err.Error())
	}

	// Use tar if running on windows
	// TODO: Figure out how to use rsync in windows so that I/O can be
	// redirected from the openshift native command to the cygwin rsync command
	if runtime.GOOS == "windows" {
		o.UseTar = true
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.Namespace = namespace

	// Determine the Rsh command to use for rsync
	if !o.UseTar {
		rsh := siblingCommand(cmd, "rsh")
		rshCmd := []string{rsh, "-n", o.Namespace}
		if len(o.ContainerName) > 0 {
			rshCmd = append(rshCmd, "-c", o.ContainerName)
		}
		o.RshCommand = strings.Join(rshCmd, " ")
		glog.V(4).Infof("Rsh command: %s", o.RshCommand)
	}

	config, err := f.ClientConfig()
	if err != nil {
		return err
	}

	client, err := f.Client()
	if err != nil {
		return err
	}

	o.RemoteExecutor = &defaultRemoteExecutor{
		Namespace:     o.Namespace,
		PodName:       o.PodName,
		ContainerName: o.ContainerName,
		Config:        config,
		Client:        client,
	}

	o.PodClient = client.Pods(namespace)

	return nil
}

// sibling command returns a sibling command to the current command
func siblingCommand(cmd *cobra.Command, name string) string {
	c := cmd.Parent()
	command := []string{}
	for c != nil {
		glog.V(5).Infof("Found parent command: %s", c.Name())
		command = append([]string{c.Name()}, command...)
		c = c.Parent()
	}
	// Replace the root command with what was actually used
	// in the command line
	glog.V(4).Infof("Setting root command to: %s", os.Args[0])
	command[0] = os.Args[0]

	// Append the sibling command
	command = append(command, name)
	glog.V(4).Infof("The sibling command is: %s", strings.Join(command, " "))

	return strings.Join(command, " ")
}

// Validate checks that SyncOptions has all necessary fields
func (o *RsyncOptions) Validate() error {
	if len(o.PodName) == 0 {
		return fmt.Errorf("pod name must be provided")
	}
	if len(o.Source) == 0 {
		return fmt.Errorf("local source must be provided")
	}
	if len(o.Destination) == 0 {
		return fmt.Errorf("remote destination must be provided")
	}
	if o.UseTar && len(o.DestinationDir) == 0 {
		return fmt.Errorf("destination directory must be provided if using tar")
	}
	if !o.UseTar && len(o.RshCommand) == 0 {
		return fmt.Errorf("rsh command must be provided when not using tar")
	}
	if o.Out == nil || o.ErrOut == nil {
		return fmt.Errorf("output and error streams must be specified")
	}
	if o.LocalExecutor == nil || o.RemoteExecutor == nil {
		return fmt.Errorf("local and remote executors must be provided")
	}
	if o.PodClient == nil {
		return fmt.Errorf("pod client must be provided")
	}
	return nil
}

func (o *RsyncOptions) copyWithRsync(out, errOut io.Writer) error {
	glog.V(3).Infof("Copying files with rsync")
	flags := "-a"
	if !o.Quiet {
		flags += "v"
	}
	cmd := []string{defaultRsyncExecutable, flags}
	if o.Delete {
		cmd = append(cmd, "--delete")
	}
	cmd = append(cmd, "-e", o.RshCommand, o.Source, o.Destination)
	glog.V(4).Infof("Local command: %s", strings.Join(cmd, " "))
	return o.LocalExecutor.Execute(cmd, nil, out, errOut)
}

func (o *RsyncOptions) copyWithTar(out, errOut io.Writer) error {
	glog.V(3).Infof("Copying files with tar")
	if o.Delete {
		// Implement the rsync --delete flag as a separate call to first delete directory contents
		deleteCmd := []string{"sh", "-c", fmt.Sprintf("rm -rf %s", filepath.Join(o.DestinationDir, "*"))}
		err := executeWithLogging(o.RemoteExecutor, deleteCmd)
		if err != nil {
			return fmt.Errorf("unable to delete files in destination: %v", err)
		}
	}
	tmp, err := ioutil.TempFile("", "rsync")
	if err != nil {
		return fmt.Errorf("cannot create local temporary file for tar: %v", err)
	}
	defer os.Remove(tmp.Name())

	err = tarLocal(o.Tar, o.Source, tmp)
	if err != nil {
		return fmt.Errorf("error creating tar of source directory: %v", err)
	}
	err = tmp.Close()
	if err != nil {
		return fmt.Errorf("error closing temporary tar file %s: %v", tmp.Name(), err)
	}
	tmp, err = os.Open(tmp.Name())
	if err != nil {
		return fmt.Errorf("cannot open temporary tar file %s: %v", tmp.Name(), err)
	}
	flags := "x"
	if !o.Quiet {
		flags += "v"
	}
	remoteCmd := []string{defaultTarExecutable, flags, "-C", o.DestinationDir}
	errBuf := &bytes.Buffer{}
	return o.RemoteExecutor.Execute(remoteCmd, tmp, out, errBuf)
}

func tarLocal(tar tar.Tar, sourceDir string, w io.Writer) error {
	glog.V(4).Infof("Tarring %s locally", sourceDir)
	// includeParent mimics rsync's behavior. When the source path ends in a path
	// separator, then only the contents of the directory are copied. Otherwise,
	// the directory itself is copied.
	includeParent := true
	if strings.HasSuffix(sourceDir, string(filepath.Separator)) {
		includeParent = false
		sourceDir = sourceDir[:len(sourceDir)-1]
	}
	return tar.CreateTarStream(sourceDir, includeParent, w)
}

// RunRsync copies files from source to destination
func (o *RsyncOptions) RunRsync() error {
	// If not going straight to tar and rsync exists locally, attempt to copy with rsync
	if !o.UseTar && o.checkLocalRsync() {
		errBuf := &bytes.Buffer{}
		err := o.copyWithRsync(o.Out, errBuf)
		// If no error occurred, we're done
		if err == nil {
			return nil
		}
		// If an error occurred, check whether rsync exists on the container.
		// If it doesn't, fallback to tar
		if o.checkRemoteRsync() {
			// If remote rsync does exist, simply report the error
			io.Copy(o.ErrOut, errBuf)
			return err
		}
	}
	return o.copyWithTar(o.Out, o.ErrOut)
}

func executeWithLogging(e executor, cmd []string) error {
	w := &bytes.Buffer{}
	err := e.Execute(cmd, nil, w, w)
	glog.V(4).Infof("%s", w.String())
	glog.V(4).Infof("error: %v", err)
	return err
}

func (o *RsyncOptions) checkLocalRsync() bool {
	_, err := exec.LookPath("rsync")
	if err != nil {
		glog.Warningf("rsync not found in local computer")
		return false
	}
	return true
}

func (o *RsyncOptions) checkRemoteRsync() bool {
	err := executeWithLogging(o.RemoteExecutor, testRsyncCommand)
	if err != nil {
		glog.Warningf("rsync not found in container %s of pod %s", o.containerName(), o.PodName)
		return false
	}
	return true
}

func (o *RsyncOptions) containerName() string {
	if len(o.ContainerName) > 0 {
		return o.ContainerName
	}
	pod, err := o.PodClient.Get(o.PodName)
	if err != nil {
		glog.V(1).Infof("Error getting pod %s: %v", o.PodName, err)
		return "[unknown]"
	}
	return pod.Spec.Containers[0].Name
}

// defaultLocalExecutor will execute commands on the local machine
type defaultLocalExecutor struct{}

// Execute will run a command locally
func (*defaultLocalExecutor) Execute(command []string, in io.Reader, out, errOut io.Writer) error {
	glog.V(3).Infof("Local executor running command: %s", strings.Join(command, " "))
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = out
	cmd.Stderr = errOut
	cmd.Stdin = in
	err := cmd.Run()
	if err != nil {
		glog.V(4).Infof("Error from local command execution: %v", err)
	}
	return err
}

// defaultRemoteExecutor will execute commands on a given pod/container by using the kube Exec command
type defaultRemoteExecutor struct {
	Namespace     string
	PodName       string
	ContainerName string
	Client        *kclient.Client
	Config        *kclient.Config
}

// Execute will run a command in a pod
func (e *defaultRemoteExecutor) Execute(command []string, in io.Reader, out, errOut io.Writer) error {
	glog.V(3).Infof("Remote executor running command: %s", strings.Join(command, " "))
	execOptions := &kubecmd.ExecOptions{
		In:            in,
		Out:           out,
		Err:           errOut,
		Stdin:         in != nil,
		Executor:      &kubecmd.DefaultRemoteExecutor{},
		Client:        e.Client,
		Config:        e.Config,
		PodName:       e.PodName,
		ContainerName: e.ContainerName,
		Namespace:     e.Namespace,
		Command:       command,
	}
	err := execOptions.Validate()
	if err != nil {
		glog.V(4).Infof("Error from remote command validation: %v", err)
		return err
	}
	err = execOptions.Run()
	if err != nil {
		glog.V(4).Infof("Error from remote execution: %v", err)
	}
	return err
}
