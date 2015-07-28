package cmd

import (
	"fmt"
	"io"
	"os"

	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"
	"github.com/spf13/cobra"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	newcmd "github.com/openshift/origin/pkg/generate/app/cmd"
)

const (
	newBuildLong = `
Create a new build in OpenShift by specifying source code

This command will try to create a build configuration for your application using images and
code that has a public repository. It will lookup the images on the local Docker installation
(if available), a Docker registry, or an OpenShift image stream.
If you specify a source code URL, it will set up a build that takes your source code and converts
it into an image that can run inside of a pod. Local source must be in a git repository that has a
remote repository that the OpenShift instance can see.

Once the build configuration is created you may need to run a build with 'start-build'.`

	newBuildExample = `  // Create a build config based on the source code in the current git repository (with a public remote) and a Docker image
  $ %[1]s new-build . --docker-image=repo/langimage

  // Create a NodeJS build config based on the provided [image]~[source code] combination
  $ %[1]s new-build openshift/nodejs-010-centos7~https://bitbucket.com/user/nodejs-app

  // Create a build config from a remote repository using its beta2 branch
  $ %[1]s new-build https://github.com/openshift/ruby-hello-world#beta2`
)

// NewCmdNewBuild implements the OpenShift cli new-build command
func NewCmdNewBuild(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	mapper, typer := f.Object()
	clientMapper := f.ClientMapperForCommand()
	config := newcmd.NewAppConfig(typer, mapper, clientMapper)

	cmd := &cobra.Command{
		Use:     "new-build (IMAGE | IMAGESTREAM | PATH | URL ...)",
		Short:   "Create a new build configuration",
		Long:    newBuildLong,
		Example: fmt.Sprintf(newBuildExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			err := RunNewBuild(fullName, f, out, c, args, config)
			if err == errExit {
				os.Exit(1)
			}
			cmdutil.CheckErr(err)
		},
	}

	cmd.Flags().Var(&config.SourceRepositories, "code", "Source code in the build configuration.")
	cmd.Flags().VarP(&config.ImageStreams, "image", "i", "Name of an OpenShift image stream to to use as a builder.")
	cmd.Flags().Var(&config.DockerImages, "docker-image", "Name of a Docker image to use as a builder.")
	cmd.Flags().StringVar(&config.Name, "name", "", "Set name to use for generated build artifacts")
	cmd.Flags().StringVar(&config.Strategy, "strategy", "", "Specify the build strategy to use if you don't want to detect (docker|source).")
	cmd.Flags().BoolVar(&config.OutputDocker, "to-docker", false, "Force the Build output to be DockerImage.")
	cmd.Flags().StringP("labels", "l", "", "Label to set in all generated resources.")
	cmdutil.AddPrinterFlags(cmd)

	return cmd
}

// RunNewBuild contains all the necessary functionality for the OpenShift cli new-build command
func RunNewBuild(fullName string, f *clientcmd.Factory, out io.Writer, c *cobra.Command, args []string, config *newcmd.AppConfig) error {
	if err := setupAppConfig(f, c, args, config); err != nil {
		return err
	}

	if err := setAppConfigLabels(c, config); err != nil {
		return err
	}
	result, err := config.RunBuilds(out, c.Out())
	if err != nil {
		if errs, ok := err.(errors.Aggregate); ok {
			if len(errs.Errors()) == 1 {
				err = errs.Errors()[0]
			}
		}
		if err == newcmd.ErrNoInputs {
			// TODO: suggest things to the user
			return cmdutil.UsageError(c, "You must specify one or more images, image streams and source code locations to create a build configuration.")
		}
		return err
	}
	if err := setLabels(config.Labels, result); err != nil {
		return err
	}
	if len(cmdutil.GetFlagString(c, "output")) != 0 {
		return f.Factory.PrintObject(c, result.List, out)
	}
	if err := createObjects(f, out, result); err != nil {
		return err
	}

	for _, item := range result.List.Items {
		switch t := item.(type) {
		case *buildapi.BuildConfig:
			fmt.Fprintf(c.Out(), "A build configuration was created - you can run `%s start-build %s` to start it.\n", fullName, t.Name)
		}
	}

	return nil
}
