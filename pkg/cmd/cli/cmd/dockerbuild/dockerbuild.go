package dockerbuild

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/credentialprovider"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util/interrupt"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/util/docker/dockerfile/builder"
)

const (
	dockerbuildLong = `
Build a Dockerfile into a single layer

Builds the provided directory with a Dockerfile into a single layered image.
Requires that you have a working connection to a Docker engine.`

	dockerbuildExample = `  # Build the current directory into a single layer and tag
  %[1]s dockerbuild . myimage:latest`
)

type DockerbuildOptions struct {
	Out io.Writer
	Err io.Writer

	Client *docker.Client

	Directory      string
	Tag            string
	DockerfilePath string
	AllowPull      bool
	Keyring        credentialprovider.DockerKeyring
	Arguments      cmdutil.Environment
}

func NewCmdDockerbuild(fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	options := &DockerbuildOptions{
		Out: out,
		Err: errOut,
	}
	cmd := &cobra.Command{
		Use:     "dockerbuild DIRECTORY TAG [--dockerfile=PATH]",
		Short:   "Perform a direct Docker build",
		Long:    dockerbuildLong,
		Example: fmt.Sprintf(dockerbuildExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			if err := options.Run(); err != nil {
				// TODO: move met to kcmdutil
				if err == cmdutil.ErrExit {
					os.Exit(1)
				}
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.DockerfilePath, "dockerfile", options.DockerfilePath, "An optional path to a Dockerfile to use.")
	cmd.Flags().BoolVar(&options.AllowPull, "allow-pull", true, "Pull the images that are not present.")
	cmd.MarkFlagFilename("dockerfile")

	return cmd
}

func (o *DockerbuildOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	paths, envArgs, ok := cmdutil.SplitEnvironmentFromResources(args)
	if !ok {
		return kcmdutil.UsageError(cmd, "context directory must be specified before environment changes: %s", strings.Join(args, " "))
	}
	if len(paths) != 2 {
		return kcmdutil.UsageError(cmd, "the directory to build and tag must be specified")
	}
	o.Arguments, _, _ = cmdutil.ParseEnvironmentArguments(envArgs)
	o.Directory = paths[0]
	o.Tag = paths[1]
	if len(o.DockerfilePath) == 0 {
		o.DockerfilePath = filepath.Join(o.Directory, "Dockerfile")
	}
	client, err := docker.NewClientFromEnv()
	if err != nil {
		return err
	}
	o.Client = client

	o.Keyring = credentialprovider.NewDockerKeyring()

	return nil
}

func (o *DockerbuildOptions) Validate() error {
	return nil
}

func (o *DockerbuildOptions) Run() error {
	f, err := os.Open(o.DockerfilePath)
	if err != nil {
		return err
	}
	defer f.Close()
	e := builder.NewClientExecutor(o.Client)
	e.Out, e.ErrOut = o.Out, o.Err
	e.AllowPull = o.AllowPull
	e.Directory = o.Directory
	e.Tag = o.Tag
	e.AuthFn = o.Keyring.Lookup
	e.LogFn = func(format string, args ...interface{}) {
		if glog.V(2) {
			glog.Infof("Builder: "+format, args...)
		} else {
			fmt.Fprintf(e.ErrOut, "--> %s\n", fmt.Sprintf(format, args...))
		}
	}
	safe := interrupt.New(func(os.Signal) { os.Exit(1) }, func() {
		glog.V(5).Infof("invoking cleanup")
		if err := e.Cleanup(); err != nil {
			fmt.Fprintf(o.Err, "error: Unable to clean up build: %v\n", err)
		}
	})
	return safe.Run(func() error { return stripLeadingError(e.Build(f, o.Arguments)) })
}

func stripLeadingError(err error) error {
	if err == nil {
		return err
	}
	if strings.HasPrefix(err.Error(), "Error: ") {
		return fmt.Errorf(strings.TrimPrefix(err.Error(), "Error: "))
	}
	return err
}
