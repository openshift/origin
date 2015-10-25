package cmd

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	ctl "k8s.io/kubernetes/pkg/kubectl"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	dockerutil "github.com/openshift/origin/pkg/cmd/util/docker"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	newapp "github.com/openshift/origin/pkg/generate/app"
	newcmd "github.com/openshift/origin/pkg/generate/app/cmd"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/util"
)

type usage interface {
	UsageError(commandName string) string
}

var errExit = fmt.Errorf("exit directly")

const (
	newAppLong = `
Create a new application by specifying source code, templates, and/or images

This command will try to build up the components of an application using images, templates,
or code that has a public repository. It will lookup the images on the local Docker installation
(if available), a Docker registry, an integrated image stream, or stored templates.

If you specify a source code URL, it will set up a build that takes your source code and converts
it into an image that can run inside of a pod. Local source must be in a git repository that has a
remote repository that the server can see. The images will be deployed via a deployment
configuration, and a service will be connected to the first public port of the app. You may either specify
components using the various existing flags or let new-app autodetect what kind of components
you have provided.

If you provide source code, a new build will be automatically triggered.
You can use '%[1]s status' to check the progress.`

	newAppExample = `
  # List all local templates and image streams that can be used to create an app
  $ %[1]s new-app --list

  # Search all templates, image streams, and Docker images for the ones that match "ruby"
  $ %[1]s new-app --search ruby

  # Create an application based on the source code in the current git repository (with a public remote)
  # and a Docker image
  $ %[1]s new-app . --docker-image=repo/langimage

  # Create a Ruby application based on the provided [image]~[source code] combination
  $ %[1]s new-app openshift/ruby-20-centos7~https://github.com/openshift/ruby-hello-world.git

  # Use the public Docker Hub MySQL image to create an app. Generated artifacts will be labeled with db=mysql
  $ %[1]s new-app mysql MYSQL_USER=user MYSQL_PASSWORD=pass MYSQL_DATABASE=testdb -l db=mysql

  # Use a MySQL image in a private registry to create an app and override application artifacts' names
  $ %[1]s new-app --docker-image=myregistry.com/mycompany/mysql --name=private

  # Create an application from a remote repository using its beta4 branch
  $ %[1]s new-app https://github.com/openshift/ruby-hello-world#beta4

  # Create an application based on a stored template, explicitly setting a parameter value
  $ %[1]s new-app --template=ruby-helloworld-sample --param=MYSQL_USER=admin

  # Create an application from a remote repository and specify a context directory
  $ %[1]s new-app https://github.com/youruser/yourgitrepo --context-dir=src/build

  # Create an application based on a template file, explicitly setting a parameter value
  $ %[1]s new-app --file=./example/myapp/template.json --param=MYSQL_USER=admin

  # Search for "mysql" in all image repositories and stored templates
  $ %[1]s new-app --search mysql

  # Search for "ruby", but only in stored templates (--template, --image and --docker-image
  # can be used to filter search results)
  $ %[1]s new-app --search --template=ruby

  # Search for "ruby" in stored templates and print the output as an YAML
  $ %[1]s new-app --search --template=ruby --output=yaml`

	newAppNoInput = `You must specify one or more images, image streams, templates, or source code locations to create an application.

To list all local templates and image streams, use:

  $ %[1]s new-app -L

To search templates, image streams, and Docker images that match the arguments provided, use:

  $ %[1]s new-app -S php
  $ %[1]s new-app -S --template=ruby
  $ %[1]s new-app -S --image=mysql
`
)

// NewCmdNewApplication implements the OpenShift cli new-app command
func NewCmdNewApplication(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	config := newcmd.NewAppConfig()

	cmd := &cobra.Command{
		Use:        "new-app (IMAGE | IMAGESTREAM | TEMPLATE | PATH | URL ...)",
		Short:      "Create a new application",
		Long:       fmt.Sprintf(newAppLong, fullName),
		Example:    fmt.Sprintf(newAppExample, fullName),
		SuggestFor: []string{"app", "application"},
		Run: func(c *cobra.Command, args []string) {
			mapper, typer := f.Object()
			config.SetMapper(mapper)
			config.SetTyper(typer)
			config.SetClientMapper(f.ClientMapperForCommand())

			err := RunNewApplication(fullName, f, out, c, args, config)
			if err == errExit {
				os.Exit(1)
			}
			cmdutil.CheckErr(err)
		},
	}

	cmd.Flags().Var(&config.SourceRepositories, "code", "Source code to use to build this application.")
	cmd.Flags().StringVar(&config.ContextDir, "context-dir", "", "Context directory to be used for the build.")
	cmd.Flags().VarP(&config.ImageStreams, "image", "", "Name of an image stream to use in the app. (deprecated)")
	cmd.Flags().VarP(&config.ImageStreams, "image-stream", "i", "Name of an image stream to use in the app.")
	cmd.Flags().Var(&config.DockerImages, "docker-image", "Name of a Docker image to include in the app.")
	cmd.Flags().Var(&config.Templates, "template", "Name of a stored template to use in the app.")
	cmd.Flags().VarP(&config.TemplateFiles, "file", "f", "Path to a template file to use for the app.")
	cmd.Flags().VarP(&config.TemplateParameters, "param", "p", "Specify a list of key value pairs (eg. -p FOO=BAR,BAR=FOO) to set/override parameter values in the template.")
	cmd.Flags().Var(&config.Groups, "group", "Indicate components that should be grouped together as <comp1>+<comp2>.")
	cmd.Flags().VarP(&config.Environment, "env", "e", "Specify key value pairs of environment variables to set into each container.")
	cmd.Flags().StringVar(&config.Name, "name", "", "Set name to use for generated application artifacts")
	cmd.Flags().StringVar(&config.Strategy, "strategy", "", "Specify the build strategy to use if you don't want to detect (docker|source).")
	cmd.Flags().StringP("labels", "l", "", "Label to set in all resources for this application.")
	cmd.Flags().BoolVar(&config.InsecureRegistry, "insecure-registry", false, "If true, indicates that the referenced Docker images are on insecure registries and should bypass certificate checking")
	cmd.Flags().BoolVarP(&config.AsList, "list", "L", false, "List all local templates and image streams that can be used to create.")
	cmd.Flags().BoolVarP(&config.AsSearch, "search", "S", false, "Search all templates, image streams, and Docker images that match the arguments provided.")
	cmd.Flags().BoolVar(&config.AllowMissingImages, "allow-missing-images", false, "If true, indicates that referenced Docker images that cannot be found locally or in a registry should still be used.")

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
	output := cmdutil.GetFlagString(c, "output")

	if err := setupAppConfig(f, out, c, args, config); err != nil {
		return err
	}
	if config.Querying() {
		result, err := config.RunQuery()
		if err != nil {
			return handleRunError(c, err, fullName)
		}

		if len(output) != 0 {
			return f.Factory.PrintObject(c, result.List, out)
		}

		return printHumanReadableQueryResult(result, out, fullName)
	}
	if err := setAppConfigLabels(c, config); err != nil {
		return err
	}
	result, err := config.RunAll()
	if err != nil {
		return handleRunError(c, err, fullName)
	}

	if err := setLabels(config.Labels, result); err != nil {
		return err
	}

	if err := setAnnotations(map[string]string{newcmd.GeneratedByNamespace: newcmd.GeneratedByNewApp}, result); err != nil {
		return err
	}

	if len(output) != 0 && output != "name" {
		return f.Factory.PrintObject(c, result.List, out)
	}
	if err := createObjects(f, out, output == "name", result); err != nil {
		return err
	}

	hasMissingRepo := false
	for _, item := range result.List.Items {
		switch t := item.(type) {
		case *buildapi.BuildConfig:
			if len(t.Spec.Triggers) > 0 {
				fmt.Fprintf(c.Out(), "Build scheduled for %q - use the build-logs command to track its progress.\n", t.Name)
			}
		case *imageapi.ImageStream:
			if len(t.Status.DockerImageRepository) == 0 {
				if hasMissingRepo {
					continue
				}
				hasMissingRepo = true
				fmt.Fprint(c.Out(), "WARNING: No Docker registry has been configured with the server. Automatic builds and deployments may not function.\n")
			}
		}
	}
	if len(result.List.Items) > 0 {
		fmt.Fprintf(c.Out(), "Run '%s %s' to view your app.\n", fullName, StatusRecommendedName)
	}

	return nil
}

func setAppConfigLabels(c *cobra.Command, config *newcmd.AppConfig) error {
	labelStr := cmdutil.GetFlagString(c, "labels")
	if len(labelStr) != 0 {
		var err error
		config.Labels, err = ctl.ParseLabels(labelStr)
		if err != nil {
			return err
		}
	}
	return nil
}

func setupAppConfig(f *clientcmd.Factory, out io.Writer, c *cobra.Command, args []string, config *newcmd.AppConfig) error {
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	dockerClient, _, err := dockerutil.NewHelper().GetClient()
	if err == nil {
		if err = dockerClient.Ping(); err == nil {
			config.SetDockerClient(dockerClient)
		} else {
			glog.V(4).Infof("Docker client did not respond to a ping: %v", err)
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
	config.Out = out
	config.ErrOut = c.Out()

	unknown := config.AddArguments(args)
	if len(unknown) != 0 {
		return cmdutil.UsageError(c, "Did not recognize the following arguments: %v", unknown)
	}

	if config.AllowMissingImages && config.AsSearch {
		return cmdutil.UsageError(c, "--allow-missing-images and --search are mutually exclusive.")
	}
	return nil
}

func setAnnotations(annotations map[string]string, result *newcmd.AppResult) error {
	for _, object := range result.List.Items {
		err := util.AddObjectAnnotations(object, annotations)
		if err != nil {
			return err
		}
	}
	return nil
}

func setLabels(labels map[string]string, result *newcmd.AppResult) error {
	if len(labels) == 0 {
		if len(result.Name) > 0 {
			labels = map[string]string{"app": result.Name}
		}
	}
	for _, object := range result.List.Items {
		err := util.AddObjectLabels(object, labels)
		if err != nil {
			return err
		}
	}
	return nil
}

// isInvalidTriggerError returns true if the given error is
// a validation error that contains 'invalid trigger type' in its
// error message. This error is returned from older servers that
// consider the presence of unknown trigger types to be an error.
func isInvalidTriggerError(err error) bool {
	if !kapierrors.IsInvalid(err) {
		return false
	}
	statusErr, ok := err.(*kapierrors.StatusError)
	if !ok {
		return false
	}
	return strings.Contains(statusErr.Status().Message, "invalid trigger type")
}

// retryBuildConfig determines if the given error is caused by an invalid trigger
// error on a BuildConfig. If that is the case, it will remove all triggers with a
// type that is not in the whitelist for an older server.
func retryBuildConfig(info *resource.Info, err error) runtime.Object {
	triggerTypeWhiteList := map[buildapi.BuildTriggerType]struct{}{
		buildapi.GitHubWebHookBuildTriggerType:  {},
		buildapi.GenericWebHookBuildTriggerType: {},
		buildapi.ImageChangeBuildTriggerType:    {},
	}
	if info.Mapping.Kind == "BuildConfig" && isInvalidTriggerError(err) {
		bc, ok := info.Object.(*buildapi.BuildConfig)
		if !ok {
			return nil
		}
		triggers := []buildapi.BuildTriggerPolicy{}
		for _, t := range bc.Spec.Triggers {
			if _, inList := triggerTypeWhiteList[t.Type]; inList {
				triggers = append(triggers, t)
			}
		}
		bc.Spec.Triggers = triggers
		return bc
	}
	return nil
}

func createObjects(f *clientcmd.Factory, out io.Writer, shortOutput bool, result *newcmd.AppResult) error {
	mapper, typer := f.Factory.Object()
	bulk := configcmd.Bulk{
		Mapper:            mapper,
		Typer:             typer,
		RESTClientFactory: f.Factory.RESTClient,
		After:             configcmd.NewPrintNameOrErrorAfter(mapper, shortOutput, "created", out, os.Stderr),
		// Retry is used to support previous versions of the API server that will
		// consider the presence of an unknown trigger type to be an error.
		Retry: retryBuildConfig,
	}
	if errs := bulk.Create(result.List, result.Namespace); len(errs) != 0 {
		return errExit
	}

	return nil
}

func handleRunError(c *cobra.Command, err error, fullName string) error {
	if err == nil {
		return nil
	}
	if errs, ok := err.(errors.Aggregate); ok {
		if len(errs.Errors()) == 1 {
			err = errs.Errors()[0]
		}
	}
	if err == newcmd.ErrNoInputs {
		// TODO: suggest things to the user
		return cmdutil.UsageError(c, newAppNoInput, fullName)
	}
	return err
}

func printHumanReadableQueryResult(r *newcmd.QueryResult, out io.Writer, fullName string) error {
	if len(r.Matches) == 0 {
		return fmt.Errorf("no matches found")
	}

	templates := newapp.ComponentMatches{}
	imageStreams := newapp.ComponentMatches{}
	dockerImages := newapp.ComponentMatches{}

	for _, match := range r.Matches {
		switch {
		case match.IsTemplate():
			templates = append(templates, match)
		case match.IsImage() && match.ImageStream != nil:
			imageStreams = append(imageStreams, match)
		case match.IsImage() && match.Image != nil:
			dockerImages = append(dockerImages, match)
		}
	}

	sort.Sort(newapp.ScoredComponentMatches(templates))
	sort.Sort(newapp.ScoredComponentMatches(imageStreams))
	sort.Sort(newapp.ScoredComponentMatches(dockerImages))

	if len(templates) > 0 {
		fmt.Fprintln(out, "Templates (oc new-app --template=<template>)")
		fmt.Fprintln(out, "-----")
		for _, match := range templates {
			template := match.Template
			description := template.ObjectMeta.Annotations["description"]

			fmt.Fprintln(out, template.Name)
			fmt.Fprintf(out, "  Project: %v\n", template.Namespace)
			if len(description) > 0 {
				fmt.Fprintf(out, "  %v\n", description)
			}
		}
		fmt.Fprintln(out)
	}

	if len(imageStreams) > 0 {
		fmt.Fprintln(out, "Image streams (oc new-app --image-stream=<image-stream> [--code=<source>])")
		fmt.Fprintln(out, "-----")
		for _, match := range imageStreams {
			imageStream := match.ImageStream
			description := imageStream.ObjectMeta.Annotations["description"]
			tags := "<none>"
			if len(imageStream.Status.Tags) > 0 {
				set := sets.NewString()
				for tag := range imageStream.Status.Tags {
					set.Insert(tag)
				}
				tags = strings.Join(set.List(), ",")
			}

			fmt.Fprintln(out, imageStream.Name)
			fmt.Fprintf(out, "  Project: %v\n", imageStream.Namespace)
			if len(imageStream.Spec.DockerImageRepository) > 0 {
				fmt.Fprintf(out, "  Tracks:  %v\n", imageStream.Spec.DockerImageRepository)
			}
			fmt.Fprintf(out, "  Tags:    %v\n", tags)
			if len(description) > 0 {
				fmt.Fprintf(out, "  %v\n", description)
			}
		}
		fmt.Fprintln(out)
	}

	if len(dockerImages) > 0 {
		fmt.Fprintln(out, "Docker images (oc new-app --docker-image=<docker-image> [--code=<source>])")
		fmt.Fprintln(out, "-----")
		for _, match := range dockerImages {
			image := match.Image

			name, tag, ok := imageapi.SplitImageStreamTag(match.Name)
			if !ok {
				name = match.Name
				tag = match.ImageTag
			}

			fmt.Fprintln(out, name)
			fmt.Fprintf(out, "  Registry: %v\n", match.Meta["registry"])
			fmt.Fprintf(out, "  Tags:     %v\n", tag)

			if len(image.Comment) > 0 {
				fmt.Fprintf(out, "  %v\n", image.Comment)
			}
		}
		fmt.Fprintln(out)
	}

	return nil
}
