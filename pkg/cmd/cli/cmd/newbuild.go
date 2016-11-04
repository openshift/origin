package cmd

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/errors"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	newapp "github.com/openshift/origin/pkg/generate/app"
	newcmd "github.com/openshift/origin/pkg/generate/app/cmd"
)

// NewBuildRecommendedCommandName is the recommended command name.
const NewBuildRecommendedCommandName = "new-build"

var (
	newBuildLong = templates.LongDesc(`
		Create a new build by specifying source code

		This command will try to create a build configuration for your application using images and
		code that has a public repository. It will lookup the images on the local Docker installation
		(if available), a Docker registry, or an image stream.

		If you specify a source code URL, it will set up a build that takes your source code and converts
		it into an image that can run inside of a pod. Local source must be in a git repository that has a
		remote repository that the server can see.

		Once the build configuration is created a new build will be automatically triggered.
		You can use '%[1]s status' to check the progress.`)

	newBuildExample = templates.Examples(`
	  # Create a build config based on the source code in the current git repository (with a public
	  # remote) and a Docker image
	  %[1]s %[2]s . --docker-image=repo/langimage

	  # Create a NodeJS build config based on the provided [image]~[source code] combination
	  %[1]s %[2]s openshift/nodejs-010-centos7~https://github.com/openshift/nodejs-ex.git

	  # Create a build config from a remote repository using its beta2 branch
	  %[1]s %[2]s https://github.com/openshift/ruby-hello-world#beta2

	  # Create a build config using a Dockerfile specified as an argument
	  %[1]s %[2]s -D $'FROM centos:7\nRUN yum install -y httpd'

	  # Create a build config from a remote repository and add custom environment variables
	  %[1]s %[2]s https://github.com/openshift/ruby-hello-world RACK_ENV=development

	  # Create a build config from a remote repository and inject the npmrc into a build
	  %[1]s %[2]s https://github.com/openshift/ruby-hello-world --build-secret npmrc:.npmrc

	  # Create a build config that gets its input from a remote repository and another Docker image
	  %[1]s %[2]s https://github.com/openshift/ruby-hello-world --source-image=openshift/jenkins-1-centos7 --source-image-path=/var/lib/jenkins:tmp`)

	newBuildNoInput = `You must specify one or more images, image streams, or source code locations to create a build.

To build from an existing image stream tag or Docker image, provide the name of the image and
the source code location:

  %[1]s %[2]s openshift/nodejs-010-centos7~https://github.com/openshift/nodejs-ex.git

If you only specify the source repository location (local or remote), the command will look at
the repo to determine the type, and then look for a matching image on your server or on the
default Docker registry.

  %[1]s %[2]s https://github.com/openshift/nodejs-ex.git

will look for an image called "nodejs" in your current project, the 'openshift' project, or
on the Docker Hub.
`
)

type NewBuildOptions struct {
	Action configcmd.BulkAction
	Config *newcmd.AppConfig

	BaseName    string
	CommandPath string
	CommandName string

	Out, ErrOut   io.Writer
	Output        string
	PrintObject   func(obj runtime.Object) error
	LogsForObject LogsForObjectFunc
}

// NewCmdNewBuild implements the OpenShift cli new-build command
func NewCmdNewBuild(name, baseName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	config := newcmd.NewAppConfig()
	config.ExpectToBuild = true
	config.AddEnvironmentToBuild = true
	o := &NewBuildOptions{Config: config}

	cmd := &cobra.Command{
		Use:        fmt.Sprintf("%s (IMAGE | IMAGESTREAM | PATH | URL ...)", name),
		Short:      "Create a new build configuration",
		Long:       fmt.Sprintf(newBuildLong, baseName, name),
		Example:    fmt.Sprintf(newBuildExample, baseName, name),
		SuggestFor: []string{"build", "builds"},
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(baseName, name, f, c, args, out, errout, in))
			err := o.RunNewBuild()
			if err == cmdutil.ErrExit {
				os.Exit(1)
			}
			kcmdutil.CheckErr(err)
		},
	}

	cmd.Flags().StringSliceVar(&config.SourceRepositories, "code", config.SourceRepositories, "Source code in the build configuration.")
	cmd.Flags().StringSliceVarP(&config.ImageStreams, "image", "", config.ImageStreams, "Name of an image stream to to use as a builder. (deprecated)")
	cmd.Flags().MarkDeprecated("image", "use --image-stream instead")
	cmd.Flags().StringSliceVarP(&config.ImageStreams, "image-stream", "i", config.ImageStreams, "Name of an image stream to to use as a builder.")
	cmd.Flags().StringSliceVar(&config.DockerImages, "docker-image", config.DockerImages, "Name of a Docker image to use as a builder.")
	cmd.Flags().StringSliceVar(&config.Secrets, "build-secret", config.Secrets, "Secret and destination to use as an input for the build.")
	cmd.Flags().StringVar(&config.Name, "name", "", "Set name to use for generated build artifacts.")
	cmd.Flags().StringVar(&config.To, "to", "", "Push built images to this image stream tag (or Docker image repository if --to-docker is set).")
	cmd.Flags().BoolVar(&config.OutputDocker, "to-docker", false, "Have the build output push to a Docker repository.")
	cmd.Flags().StringArrayVarP(&config.Environment, "env", "e", config.Environment, "Specify a key-value pair for an environment variable to set into resulting image.")
	cmd.Flags().StringVar(&config.Strategy, "strategy", "", "Specify the build strategy to use if you don't want to detect (docker|source).")
	cmd.Flags().StringVarP(&config.Dockerfile, "dockerfile", "D", "", "Specify the contents of a Dockerfile to build directly, implies --strategy=docker. Pass '-' to read from STDIN.")
	cmd.Flags().BoolVar(&config.BinaryBuild, "binary", false, "Instead of expecting a source URL, set the build to expect binary contents. Will disable triggers.")
	cmd.Flags().StringP("labels", "l", "", "Label to set in all generated resources.")
	cmd.Flags().BoolVar(&config.AllowMissingImages, "allow-missing-images", false, "If true, indicates that referenced Docker images that cannot be found locally or in a registry should still be used.")
	cmd.Flags().BoolVar(&config.AllowMissingImageStreamTags, "allow-missing-imagestream-tags", false, "If true, indicates that image stream tags that don't exist should still be used.")
	cmd.Flags().StringVar(&config.ContextDir, "context-dir", "", "Context directory to be used for the build.")
	cmd.Flags().BoolVar(&config.NoOutput, "no-output", false, "If true, the build output will not be pushed anywhere.")
	cmd.Flags().StringVar(&config.SourceImage, "source-image", "", "Specify an image to use as source for the build.  You must also specify --source-image-path.")
	cmd.Flags().StringVar(&config.SourceImagePath, "source-image-path", "", "Specify the file or directory to copy from the source image and its destination in the build directory. Format: [source]:[destination-dir].")

	o.Action.BindForOutput(cmd.Flags())
	cmd.Flags().String("output-version", "", "The preferred API versions of the output objects")

	return cmd
}

// Complete sets any default behavior for the command
func (o *NewBuildOptions) Complete(baseName, commandName string, f *clientcmd.Factory, c *cobra.Command, args []string, out, errout io.Writer, in io.Reader) error {
	o.Out = out
	o.ErrOut = errout
	o.Output = kcmdutil.GetFlagString(c, "output")
	// Only output="" should print descriptions of intermediate steps. Everything
	// else should print only some specific output (json, yaml, go-template, ...)
	if len(o.Output) == 0 {
		o.Config.Out = o.Out
	} else {
		o.Config.Out = ioutil.Discard
	}
	o.Config.ErrOut = o.ErrOut

	o.Action.Out, o.Action.ErrOut = o.Out, o.ErrOut
	o.Action.Bulk.Mapper = clientcmd.ResourceMapper(f)
	o.Action.Bulk.Op = configcmd.Create
	// Retry is used to support previous versions of the API server that will
	// consider the presence of an unknown trigger type to be an error.
	o.Action.Bulk.Retry = retryBuildConfig

	o.Config.DryRun = o.Action.DryRun
	o.Config.AllowNonNumericExposedPorts = true

	o.BaseName = baseName
	o.CommandPath = c.CommandPath()
	o.CommandName = commandName

	cmdutil.WarnAboutCommaSeparation(o.ErrOut, o.Config.Environment, "--env")

	mapper, _ := f.Object(false)
	o.PrintObject = cmdutil.VersionedPrintObject(f.PrintObject, c, mapper, out)
	o.LogsForObject = f.LogsForObject
	if err := CompleteAppConfig(o.Config, f, c, args); err != nil {
		return err
	}
	if o.Config.Dockerfile == "-" {
		data, err := ioutil.ReadAll(in)
		if err != nil {
			return err
		}
		o.Config.Dockerfile = string(data)
	}
	if err := setAppConfigLabels(c, o.Config); err != nil {
		return err
	}
	return nil
}

// RunNewBuild contains all the necessary functionality for the OpenShift cli new-build command
func (o *NewBuildOptions) RunNewBuild() error {
	config := o.Config
	out := o.Out

	checkGitInstalled(out)

	result, err := config.Run()
	if err != nil {
		return handleBuildError(err, o.BaseName, o.CommandName, o.CommandPath)
	}

	if len(config.Labels) == 0 && len(result.Name) > 0 {
		config.Labels = map[string]string{"build": result.Name}
	}

	if err := setLabels(config.Labels, result); err != nil {
		return err
	}
	if err := setAnnotations(map[string]string{newcmd.GeneratedByNamespace: newcmd.GeneratedByNewBuild}, result); err != nil {
		return err
	}

	if o.Action.ShouldPrint() {
		return o.PrintObject(result.List)
	}

	if errs := o.Action.WithMessage(configcmd.CreateMessage(config.Labels), "created").Run(result.List, result.Namespace); len(errs) > 0 {
		return cmdutil.ErrExit
	}

	if !o.Action.Verbose() || o.Action.DryRun {
		return nil
	}

	indent := o.Action.DefaultIndent()
	for _, item := range result.List.Items {
		switch t := item.(type) {
		case *buildapi.BuildConfig:
			if len(t.Spec.Triggers) > 0 && t.Spec.Source.Binary == nil {
				fmt.Fprintf(out, "%sBuild configuration %q created and build triggered.\n", indent, t.Name)
				fmt.Fprintf(out, "%sRun '%s logs -f bc/%s' to stream the build progress.\n", indent, o.BaseName, t.Name)
			}
		}
	}

	return nil
}

func handleBuildError(err error, baseName, commandName, commandPath string) error {
	if err == nil {
		return nil
	}
	errs := []error{err}
	if agg, ok := err.(errors.Aggregate); ok {
		errs = agg.Errors()
	}
	groups := errorGroups{}
	for _, err := range errs {
		transformBuildError(err, baseName, commandName, commandPath, groups)
	}
	buf := &bytes.Buffer{}
	for _, group := range groups {
		fmt.Fprint(buf, kcmdutil.MultipleErrors("error: ", group.errs))
		if len(group.suggestion) > 0 {
			fmt.Fprintln(buf)
		}
		fmt.Fprint(buf, group.suggestion)
	}
	return fmt.Errorf(buf.String())
}

func transformBuildError(err error, baseName, commandName, commandPath string, groups errorGroups) {
	switch t := err.(type) {
	case newapp.ErrNoMatch:
		groups.Add(
			"no-matches",
			heredoc.Docf(`
				The '%[1]s' command will match arguments to the following types:

				  1. Images tagged into image streams in the current project or the 'openshift' project
				     - if you don't specify a tag, we'll add ':latest'
				  2. Images in the Docker Hub, on remote registries, or on the local Docker engine
				  3. Git repository URLs or local paths that point to Git repositories

				--allow-missing-images can be used to force the use of an image that was not matched

				See '%[1]s -h' for examples.`, commandPath,
			),
			t,
			t.Errs...,
		)
		return
	}
	switch err {
	case newcmd.ErrNoInputs:
		groups.Add("", "", usageError(commandPath, newBuildNoInput, baseName, commandName))
		return
	}
	transformError(err, baseName, commandName, commandPath, groups)
}
