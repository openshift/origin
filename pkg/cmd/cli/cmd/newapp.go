package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	ctl "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	dockerutil "github.com/openshift/origin/pkg/cmd/util/docker"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	newcmd "github.com/openshift/origin/pkg/generate/app/cmd"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/util"
)

type usage interface {
	UsageError(commandName string) string
}

var errExit = fmt.Errorf("exit directly")

const (
	newAppLong = `Create a new application in OpenShift by specifying source code, templates, and/or images.

This command will try to build up the components of an application using images, templates,
or code that has a public repository. It will lookup the images on the local Docker installation
(if available), a Docker registry, or an OpenShift image stream.
If you specify a source code URL, it will set up a build that takes your source code and converts
it into an image that can run inside of a pod. Local source must be in a git repository that has a
remote repository that the OpenShift instance can see. The images will be deployed via a deployment
configuration, and a service will be connected to the first public port of the app. You may either specify
components using the various existing flags or let new-app autodetect what kind of components
you have provided.

If you provide source code, you may need to run a build with 'start-build' after the
application is created.`

	newAppExample = `  // Create an application based on the source code in the current git repository (with a public remote) and a Docker image
  $ %[1]s new-app . --docker-image=repo/langimage

  // Create a Ruby application based on the provided [image]~[source code] combination
  $ %[1]s new-app openshift/ruby-20-centos7~https://github.com/openshift/ruby-hello-world.git

  // Use the public Docker Hub MySQL image to create an app. Generated artifacts will be labeled with db=mysql
  $ %[1]s new-app mysql -l db=mysql

  // Use a MySQL image in a private registry to create an app and override application artifacts' names
  $ %[1]s new-app --docker-image=myregistry.com/mycompany/mysql --name=private

  // Create an application from a remote repository using its beta4 branch
  $ %[1]s new-app https://github.com/openshift/ruby-hello-world#beta4

  // Create an application based on a stored template, explicitly setting a parameter value
  $ %[1]s new-app --template=ruby-helloworld-sample --param=MYSQL_USER=admin

  // Create an application from a remote repository and specify a context directory
  $ %[1]s new-app https://github.com/youruser/yourgitrepo --context-dir=src/build
 
  // Create an application based on a template file, explicitly setting a parameter value
  $ %[1]s new-app --file=./example/myapp/template.json --param=MYSQL_USER=admin`
)

// NewCmdNewApplication implements the OpenShift cli new-app command
func NewCmdNewApplication(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	mapper, typer := f.Object()
	clientMapper := f.ClientMapperForCommand()
	config := newcmd.NewAppConfig(typer, mapper, clientMapper)

	cmd := &cobra.Command{
		Use:     "new-app (IMAGE | IMAGESTREAM | TEMPLATE | PATH | URL ...)",
		Short:   "Create a new application",
		Long:    newAppLong,
		Example: fmt.Sprintf(newAppExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			err := RunNewApplication(fullName, f, out, c, args, config)
			if err == errExit {
				os.Exit(1)
			}
			cmdutil.CheckErr(err)
		},
	}

	cmd.Flags().Var(&config.SourceRepositories, "code", "Source code to use to build this application.")
	cmd.Flags().StringVar(&config.ContextDir, "context-dir", "", "Context directory to be used for the build.")
	cmd.Flags().VarP(&config.ImageStreams, "image", "", "Name of an OpenShift image stream to use in the app. (deprecated)")
	cmd.Flags().VarP(&config.ImageStreams, "image-stream", "i", "Name of an OpenShift image stream to use in the app.")
	cmd.Flags().Var(&config.DockerImages, "docker-image", "Name of a Docker image to include in the app.")
	cmd.Flags().Var(&config.Templates, "template", "Name of an OpenShift stored template to use in the app.")
	cmd.Flags().VarP(&config.TemplateFiles, "file", "f", "Path to a template file to use for the app.")
	cmd.Flags().VarP(&config.TemplateParameters, "param", "p", "Specify a list of key value pairs (eg. -p FOO=BAR,BAR=FOO) to set/override parameter values in the template.")
	cmd.Flags().Var(&config.Groups, "group", "Indicate components that should be grouped together as <comp1>+<comp2>.")
	cmd.Flags().VarP(&config.Environment, "env", "e", "Specify key value pairs of environment variables to set into each container.")
	cmd.Flags().StringVar(&config.Name, "name", "", "Set name to use for generated application artifacts")
	cmd.Flags().StringVar(&config.Strategy, "strategy", "", "Specify the build strategy to use if you don't want to detect (docker|source).")
	cmd.Flags().StringP("labels", "l", "", "Label to set in all resources for this application.")
	cmd.Flags().BoolVar(&config.InsecureRegistry, "insecure-registry", false, "If true, indicates that the referenced Docker images are on insecure registries and should bypass certificate checking")

	// TODO AddPrinterFlags disabled so that it doesn't conflict with our own "template" flag.
	// Need a better solution.
	// cmdutil.AddPrinterFlags(cmd)
	cmd.Flags().StringP("output", "o", "", "Output format. One of: json|yaml|template|templatefile.")
	cmd.Flags().String("output-version", "", "Output the formatted object with the given version (default api-version).")
	cmd.Flags().Bool("no-headers", false, "When using the default output, don't print headers.")
	cmd.Flags().String("output-template", "", "Template string or path to template file to use when -o=template or -o=templatefile.  The template format is golang templates [http://golang.org/pkg/text/template/#pkg-overview]")

	return cmd
}

// RunNewApplication contains all the necessary functionality for the OpenShift cli new-app command
func RunNewApplication(fullName string, f *clientcmd.Factory, out io.Writer, c *cobra.Command, args []string, config *newcmd.AppConfig) error {
	if err := setupAppConfig(f, c, args, config); err != nil {
		return err
	}

	result, err := config.RunAll(out)
	if err != nil {
		if errs, ok := err.(errors.Aggregate); ok {
			if len(errs.Errors()) == 1 {
				err = errs.Errors()[0]
			}
		}
		if err == newcmd.ErrNoInputs {
			// TODO: suggest things to the user
			return cmdutil.UsageError(c, "You must specify one or more images, image streams, templates or source code locations to create an application.")
		}
		return err
	}

	if err := setLabels(c, result); err != nil {
		return err
	}
	if len(cmdutil.GetFlagString(c, "output")) != 0 {
		return f.Factory.PrintObject(c, result.List, out)
	}
	if err := createObjects(f, out, result); err != nil {
		return err
	}

	hasMissingRepo := false
	for _, item := range result.List.Items {
		switch t := item.(type) {
		case *kapi.Service:
			portMappings := "."
			if len(t.Spec.Ports) > 0 {
				portMappings = fmt.Sprintf(" with port mappings %s.", describeServicePorts(t.Spec))
			}
			fmt.Fprintf(c.Out(), "Service %q created at %s%s\n", t.Name, t.Spec.ClusterIP, portMappings)
		case *buildapi.BuildConfig:
			fmt.Fprintf(c.Out(), "A build was created - you can run `%s start-build %s` to start it.\n", fullName, t.Name)
		case *imageapi.ImageStream:
			if len(t.Status.DockerImageRepository) == 0 {
				if hasMissingRepo {
					continue
				}
				hasMissingRepo = true
				fmt.Fprintf(c.Out(), "WARNING: We created an ImageStream %q, but it does not look like a Docker registry has been integrated with the OpenShift server. Automatic builds and deployments depend on that integration to detect new images and will not function properly.\n", t.Name)
			}
		}
	}
	return nil
}

func setupAppConfig(f *clientcmd.Factory, c *cobra.Command, args []string, config *newcmd.AppConfig) error {
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	dockerClient, _, err := dockerutil.NewHelper().GetClient()
	if err == nil {
		if err = dockerClient.Ping(); err == nil {
			config.SetDockerClient(dockerClient)
		}
	}
	if err != nil {
		glog.V(2).Infof("No local Docker daemon detected: %v", err)
	}

	osclient, _, err := f.Clients()
	if err != nil {
		return err
	}
	config.SetOpenShiftClient(osclient, namespace)

	unknown := config.AddArguments(args)
	if len(unknown) != 0 {
		return cmdutil.UsageError(c, "Did not recognize the following arguments: %v", unknown)
	}

	return nil
}

func setLabels(c *cobra.Command, result *newcmd.AppResult) error {
	label := cmdutil.GetFlagString(c, "labels")
	if len(label) != 0 {
		lbl, err := ctl.ParseLabels(label)
		if err != nil {
			return err
		}
		for _, object := range result.List.Items {
			err = util.AddObjectLabels(object, lbl)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func createObjects(f *clientcmd.Factory, out io.Writer, result *newcmd.AppResult) error {
	// TODO: Validate everything before building
	mapper, typer := f.Factory.Object()
	bulk := configcmd.Bulk{
		Mapper:            mapper,
		Typer:             typer,
		RESTClientFactory: f.Factory.RESTClient,

		After: configcmd.NewPrintNameOrErrorAfter(out, os.Stderr),
	}
	if errs := bulk.Create(result.List, result.Namespace); len(errs) != 0 {
		return errExit
	}

	return nil
}

func describeServicePorts(spec kapi.ServiceSpec) string {
	switch len(spec.Ports) {
	case 1:
		if spec.Ports[0].TargetPort.String() == "0" || spec.ClusterIP == kapi.ClusterIPNone || spec.Ports[0].Port == spec.Ports[0].TargetPort.IntVal {
			return fmt.Sprintf("%d", spec.Ports[0].Port)
		}
		return fmt.Sprintf("%d->%s", spec.Ports[0].Port, spec.Ports[0].TargetPort.String())
	default:
		pairs := []string{}
		for _, port := range spec.Ports {
			if port.TargetPort.String() == "0" || spec.ClusterIP == kapi.ClusterIPNone {
				pairs = append(pairs, fmt.Sprintf("%d", port.Port))
				continue
			}
			pairs = append(pairs, fmt.Sprintf("%d->%s", port.Port, port.TargetPort.String()))
		}
		return strings.Join(pairs, ", ")
	}
}
