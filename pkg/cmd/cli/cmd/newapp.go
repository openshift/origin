package cmd

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/MakeNowJust/heredoc"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	ctl "k8s.io/kubernetes/pkg/kubectl"
	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	dockerutil "github.com/openshift/origin/pkg/cmd/util/docker"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	newapp "github.com/openshift/origin/pkg/generate/app"
	newcmd "github.com/openshift/origin/pkg/generate/app/cmd"
	"github.com/openshift/origin/pkg/generate/git"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/util"
)

// NewAppRecommendedCommandName is the recommended command name.
const NewAppRecommendedCommandName = "new-app"

type usage interface {
	UsageError(baseName string) string
}

var (
	newAppLong = templates.LongDesc(`
		Create a new application by specifying source code, templates, and/or images

		This command will try to build up the components of an application using images, templates,
		or code that has a public repository. It will lookup the images on the local Docker installation
		(if available), a Docker registry, an integrated image stream, or stored templates.

		If you specify a source code URL, it will set up a build that takes your source code and converts
		it into an image that can run inside of a pod. Local source must be in a git repository that has a
		remote repository that the server can see. The images will be deployed via a deployment
		configuration, and a service will be connected to the first public port of the app. You may either specify
		components using the various existing flags or let %[2]s autodetect what kind of components
		you have provided.

		If you provide source code, a new build will be automatically triggered.
		You can use '%[1]s status' to check the progress.`)

	newAppExample = templates.Examples(`
	  # List all local templates and image streams that can be used to create an app
	  %[1]s %[2]s --list

	  # Create an application based on the source code in the current git repository (with a public remote)
	  # and a Docker image
	  %[1]s %[2]s . --docker-image=repo/langimage

	  # Create a Ruby application based on the provided [image]~[source code] combination
	  %[1]s %[2]s centos/ruby-22-centos7~https://github.com/openshift/ruby-ex.git

	  # Use the public Docker Hub MySQL image to create an app. Generated artifacts will be labeled with db=mysql
	  %[1]s %[2]s mysql MYSQL_USER=user MYSQL_PASSWORD=pass MYSQL_DATABASE=testdb -l db=mysql

	  # Use a MySQL image in a private registry to create an app and override application artifacts' names
	  %[1]s %[2]s --docker-image=myregistry.com/mycompany/mysql --name=private

	  # Create an application from a remote repository using its beta4 branch
	  %[1]s %[2]s https://github.com/openshift/ruby-hello-world#beta4

	  # Create an application based on a stored template, explicitly setting a parameter value
	  %[1]s %[2]s --template=ruby-helloworld-sample --param=MYSQL_USER=admin

	  # Create an application from a remote repository and specify a context directory
	  %[1]s %[2]s https://github.com/youruser/yourgitrepo --context-dir=src/build

	  # Create an application based on a template file, explicitly setting a parameter value
	  %[1]s %[2]s --file=./example/myapp/template.json --param=MYSQL_USER=admin

	  # Search all templates, image streams, and Docker images for the ones that match "ruby"
	  %[1]s %[2]s --search ruby

	  # Search for "ruby", but only in stored templates (--template, --image-stream and --docker-image
	  # can be used to filter search results)
	  %[1]s %[2]s --search --template=ruby

	  # Search for "ruby" in stored templates and print the output as an YAML
	  %[1]s %[2]s --search --template=ruby --output=yaml`)

	newAppNoInput = `You must specify one or more images, image streams, templates, or source code locations to create an application.

To list all local templates and image streams, use:

  %[1]s %[2]s -L

To search templates, image streams, and Docker images that match the arguments provided, use:

  %[1]s %[2]s -S php
  %[1]s %[2]s -S --template=ruby
  %[1]s %[2]s -S --image-stream=mysql
  %[1]s %[2]s -S --docker-image=python
`
)

type NewAppOptions struct {
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

// NewCmdNewApplication implements the OpenShift cli new-app command.
func NewCmdNewApplication(name, baseName string, f *clientcmd.Factory, out, errout io.Writer) *cobra.Command {
	config := newcmd.NewAppConfig()
	config.Deploy = true
	o := &NewAppOptions{Config: config}

	cmd := &cobra.Command{
		Use:        fmt.Sprintf("%s (IMAGE | IMAGESTREAM | TEMPLATE | PATH | URL ...)", name),
		Short:      "Create a new application",
		Long:       fmt.Sprintf(newAppLong, baseName, name),
		Example:    fmt.Sprintf(newAppExample, baseName, name),
		SuggestFor: []string{"app", "application"},
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(baseName, name, f, c, args, out, errout))
			err := o.RunNewApp()
			if err == cmdutil.ErrExit {
				os.Exit(1)
			}
			kcmdutil.CheckErr(err)
		},
	}

	cmd.Flags().BoolVar(&config.AsTestDeployment, "as-test", config.AsTestDeployment, "If true create this application as a test deployment, which validates that the deployment succeeds and then scales down.")
	cmd.Flags().StringSliceVar(&config.SourceRepositories, "code", config.SourceRepositories, "Source code to use to build this application.")
	cmd.Flags().StringVar(&config.ContextDir, "context-dir", "", "Context directory to be used for the build.")
	cmd.Flags().StringSliceVarP(&config.ImageStreams, "image", "", config.ImageStreams, "Name of an image stream to use in the app. (deprecated)")
	cmd.Flags().MarkDeprecated("image", "use --image-stream instead")
	cmd.Flags().StringSliceVarP(&config.ImageStreams, "image-stream", "i", config.ImageStreams, "Name of an image stream to use in the app.")
	cmd.Flags().StringSliceVar(&config.DockerImages, "docker-image", config.DockerImages, "Name of a Docker image to include in the app.")
	cmd.Flags().StringSliceVar(&config.Templates, "template", config.Templates, "Name of a stored template to use in the app.")
	cmd.Flags().StringSliceVarP(&config.TemplateFiles, "file", "f", config.TemplateFiles, "Path to a template file to use for the app.")
	cmd.MarkFlagFilename("file", "yaml", "yml", "json")
	cmd.Flags().StringArrayVarP(&config.TemplateParameters, "param", "p", config.TemplateParameters, "Specify a key-value pair (e.g., -p FOO=BAR) to set/override a parameter value in the template.")
	cmd.Flags().StringSliceVar(&config.Groups, "group", config.Groups, "Indicate components that should be grouped together as <comp1>+<comp2>.")
	cmd.Flags().StringArrayVarP(&config.Environment, "env", "e", config.Environment, "Specify a key-value pair for an environment variable to set into each container. This doesn't apply to objects created from a template, use parameters instead.")
	cmd.Flags().StringVar(&config.Name, "name", "", "Set name to use for generated application artifacts")
	cmd.Flags().StringVar(&config.Strategy, "strategy", "", "Specify the build strategy to use if you don't want to detect (docker|source).")
	cmd.Flags().StringP("labels", "l", "", "Label to set in all resources for this application.")
	cmd.Flags().BoolVar(&config.InsecureRegistry, "insecure-registry", false, "If true, indicates that the referenced Docker images are on insecure registries and should bypass certificate checking")
	cmd.Flags().BoolVarP(&config.AsList, "list", "L", false, "List all local templates and image streams that can be used to create.")
	cmd.Flags().BoolVarP(&config.AsSearch, "search", "S", false, "Search all templates, image streams, and Docker images that match the arguments provided.")
	cmd.Flags().BoolVar(&config.AllowMissingImages, "allow-missing-images", false, "If true, indicates that referenced Docker images that cannot be found locally or in a registry should still be used.")
	cmd.Flags().BoolVar(&config.AllowMissingImageStreamTags, "allow-missing-imagestream-tags", false, "If true, indicates that image stream tags that don't exist should still be used.")
	cmd.Flags().BoolVar(&config.AllowSecretUse, "grant-install-rights", false, "If true, a component that requires access to your account may use your token to install software into your project. Only grant images you trust the right to run with your token.")
	cmd.Flags().BoolVar(&config.SkipGeneration, "no-install", false, "Do not attempt to run images that describe themselves as being installable")

	o.Action.BindForOutput(cmd.Flags())
	cmd.Flags().String("output-version", "", "The preferred API versions of the output objects")

	return cmd
}

// Complete sets any default behavior for the command
func (o *NewAppOptions) Complete(baseName, name string, f *clientcmd.Factory, c *cobra.Command, args []string, out, errout io.Writer) error {
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

	cmdutil.WarnAboutCommaSeparation(o.ErrOut, o.Config.Environment, "--env")
	cmdutil.WarnAboutCommaSeparation(o.ErrOut, o.Config.TemplateParameters, "--param")

	o.CommandPath = c.CommandPath()
	o.BaseName = baseName
	o.CommandName = name
	mapper, _ := f.Object(false)
	o.PrintObject = cmdutil.VersionedPrintObject(f.PrintObject, c, mapper, out)
	o.LogsForObject = f.LogsForObject
	if err := CompleteAppConfig(o.Config, f, c, args); err != nil {
		return err
	}
	if err := setAppConfigLabels(c, o.Config); err != nil {
		return err
	}
	return nil
}

// RunNewApp contains all the necessary functionality for the OpenShift cli new-app command
func (o *NewAppOptions) RunNewApp() error {
	config := o.Config
	out := o.Out

	if config.Querying() {
		result, err := config.RunQuery()
		if err != nil {
			return handleRunError(err, o.BaseName, o.CommandName, o.CommandPath)
		}

		if o.Action.ShouldPrint() {
			return o.PrintObject(result.List)
		}

		return printHumanReadableQueryResult(result, out, o.BaseName, o.CommandName)
	}

	checkGitInstalled(out)

	result, err := config.Run()
	if err := handleRunError(err, o.BaseName, o.CommandName, o.CommandPath); err != nil {
		return err
	}

	// set labels explicitly supplied by the user on the command line
	if err := setLabels(config.Labels, result); err != nil {
		return err
	}

	if len(result.Name) > 0 {
		// only set the computed implicit "app" label on objects if no object we've produced
		// already has the "app" label.
		appLabel := map[string]string{"app": result.Name}
		hasAppLabel, err := hasLabel(appLabel, result)
		if err != nil {
			return err
		}
		if !hasAppLabel {
			if err := setLabels(appLabel, result); err != nil {
				return err
			}
		}
	}

	if err := setAnnotations(map[string]string{newcmd.GeneratedByNamespace: newcmd.GeneratedByNewApp}, result); err != nil {
		return err
	}

	if o.Action.ShouldPrint() {
		return o.PrintObject(result.List)
	}

	if result.GeneratedJobs {
		o.Action.Compact()
	}

	if errs := o.Action.WithMessage(configcmd.CreateMessage(config.Labels), "created").Run(result.List, result.Namespace); len(errs) > 0 {
		return cmdutil.ErrExit
	}

	if !o.Action.Verbose() || o.Action.DryRun {
		return nil
	}

	hasMissingRepo := false
	installing := []*kapi.Pod{}
	indent := o.Action.DefaultIndent()
	for _, item := range result.List.Items {
		switch t := item.(type) {
		case *kapi.Pod:
			if t.Annotations[newcmd.GeneratedForJob] == "true" {
				installing = append(installing, t)
			}
		case *buildapi.BuildConfig:
			triggered := false
			for _, trigger := range t.Spec.Triggers {
				switch trigger.Type {
				case buildapi.ImageChangeBuildTriggerType, buildapi.ConfigChangeBuildTriggerType:
					triggered = true
					break
				}
			}
			if triggered {
				fmt.Fprintf(out, "%[1]sBuild scheduled, use '%[3]s logs -f bc/%[2]s' to track its progress.\n", indent, t.Name, o.BaseName)
			} else {
				fmt.Fprintf(out, "%[1]sUse '%[3]s start-build %[2]s' to start a build.\n", indent, t.Name, o.BaseName)
			}
		case *imageapi.ImageStream:
			if len(t.Status.DockerImageRepository) == 0 {
				if hasMissingRepo {
					continue
				}
				hasMissingRepo = true
				fmt.Fprintf(out, "%sWARNING: No Docker registry has been configured with the server. Automatic builds and deployments may not function.\n", indent)
			}
		}
	}

	switch {
	case len(installing) == 1:
		jobInput := installing[0].Annotations[newcmd.GeneratedForJobFor]
		return followInstallation(config, jobInput, installing[0], o.LogsForObject)
	case len(installing) > 1:
		for i := range installing {
			fmt.Fprintf(out, "%sTrack installation of %s with '%s logs %s'.\n", indent, installing[i].Name, o.BaseName, installing[i].Name)
		}
	case len(result.List.Items) > 0:
		fmt.Fprintf(out, "%sRun '%s %s' to view your app.\n", indent, o.BaseName, StatusRecommendedName)
	}
	return nil
}

type LogsForObjectFunc func(object, options runtime.Object) (*restclient.Request, error)

func followInstallation(config *newcmd.AppConfig, input string, pod *kapi.Pod, logsForObjectFn LogsForObjectFunc) error {
	fmt.Fprintf(config.Out, "--> Installing ...\n")

	// we cannot retrieve logs until the pod is out of pending
	// TODO: move this to the server side
	podClient := config.KubeClient.Pods(pod.Namespace)
	if err := wait.PollImmediate(500*time.Millisecond, 60*time.Second, installationStarted(podClient, pod.Name, config.KubeClient.Secrets(pod.Namespace))); err != nil {
		return err
	}

	opts := &kcmd.LogsOptions{
		Namespace:   pod.Namespace,
		ResourceArg: pod.Name,
		Options: &kapi.PodLogOptions{
			Follow:    true,
			Container: pod.Spec.Containers[0].Name,
		},
		Mapper:        config.Mapper,
		Typer:         config.Typer,
		ClientMapper:  config.ClientMapper,
		LogsForObject: logsForObjectFn,
		Out:           config.Out,
	}
	_, logErr := opts.RunLogs()

	// status of the pod may take tens of seconds to propagate
	if err := wait.PollImmediate(500*time.Millisecond, 30*time.Second, installationComplete(podClient, pod.Name, config.Out)); err != nil {
		if err == wait.ErrWaitTimeout {
			if logErr != nil {
				// output the log error if one occurred
				err = logErr
			} else {
				err = fmt.Errorf("installation may not have completed, see logs for %q for more information", pod.Name)
			}
		}
		return err
	}

	return nil
}

func installationStarted(c kclient.PodInterface, name string, s kclient.SecretsInterface) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := c.Get(name)
		if err != nil {
			return false, err
		}
		if pod.Status.Phase == kapi.PodPending {
			return false, nil
		}
		// delete a secret named the same as the pod if it exists
		if secret, err := s.Get(name); err == nil {
			if secret.Annotations[newcmd.GeneratedForJob] == "true" &&
				secret.Annotations[newcmd.GeneratedForJobFor] == pod.Annotations[newcmd.GeneratedForJobFor] {
				if err := s.Delete(name); err != nil {
					glog.V(4).Infof("Failed to delete install secret %s: %v", name, err)
				}
			}
		}
		return true, nil
	}
}

func installationComplete(c kclient.PodInterface, name string, out io.Writer) wait.ConditionFunc {
	return func() (bool, error) {
		pod, err := c.Get(name)
		if err != nil {
			if kapierrors.IsNotFound(err) {
				return false, fmt.Errorf("installation pod was deleted; unable to determine whether it completed successfully")
			}
			return false, nil
		}
		switch pod.Status.Phase {
		case kapi.PodSucceeded:
			fmt.Fprintf(out, "--> Success\n")
			if err := c.Delete(name, nil); err != nil {
				glog.V(4).Infof("Failed to delete install pod %s: %v", name, err)
			}
			return true, nil
		case kapi.PodFailed:
			return true, fmt.Errorf("installation of %q did not complete successfully", name)
		default:
			return false, nil
		}
	}
}

func setAppConfigLabels(c *cobra.Command, config *newcmd.AppConfig) error {
	labelStr := kcmdutil.GetFlagString(c, "labels")
	if len(labelStr) != 0 {
		var err error
		config.Labels, err = ctl.ParseLabels(labelStr)
		if err != nil {
			return err
		}
	}
	return nil
}

// getDockerClient returns a client capable of communicating with the local
// docker daemon.  If an error occurs (such as no local daemon being available),
// it will return nil.
func getDockerClient() (*docker.Client, error) {
	dockerClient, _, err := dockerutil.NewHelper().GetClient()
	if err == nil {
		if err = dockerClient.Ping(); err != nil {
			glog.V(4).Infof("Docker client did not respond to a ping: %v", err)
			return nil, err
		}
		return dockerClient, nil
	}
	glog.V(2).Infof("No local Docker daemon detected: %v", err)
	return nil, err
}

func CompleteAppConfig(config *newcmd.AppConfig, f *clientcmd.Factory, c *cobra.Command, args []string) error {
	mapper, typer := f.Object(false)
	if config.Mapper == nil {
		config.Mapper = mapper
	}
	if config.Typer == nil {
		config.Typer = typer
	}
	if config.ClientMapper == nil {
		config.ClientMapper = resource.ClientMapperFunc(f.ClientForMapping)
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	osclient, kclient, err := f.Clients()
	if err != nil {
		return err
	}
	config.KubeClient = kclient
	dockerClient, _ := getDockerClient()
	config.SetOpenShiftClient(osclient, namespace, dockerClient)

	if config.AllowSecretUse {
		cfg, err := f.OpenShiftClientConfig.ClientConfig()
		if err != nil {
			return err
		}
		config.SecretAccessor = newConfigSecretRetriever(cfg)
	}

	unknown := config.AddArguments(args)
	if len(unknown) != 0 {
		return kcmdutil.UsageError(c, "Did not recognize the following arguments: %v", unknown)
	}

	if config.AllowMissingImages && config.AsSearch {
		return kcmdutil.UsageError(c, "--allow-missing-images and --search are mutually exclusive.")
	}

	if len(config.SourceImage) != 0 && len(config.SourceImagePath) == 0 {
		return kcmdutil.UsageError(c, "--source-image-path must be specified when --source-image is specified.")
	}
	if len(config.SourceImage) == 0 && len(config.SourceImagePath) != 0 {
		return kcmdutil.UsageError(c, "--source-image must be specified when --source-image-path is specified.")
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
	for _, object := range result.List.Items {
		err := util.AddObjectLabels(object, labels)
		if err != nil {
			return err
		}
	}
	return nil
}

func hasLabel(labels map[string]string, result *newcmd.AppResult) (bool, error) {
	for _, obj := range result.List.Items {
		objCopy, err := kapi.Scheme.DeepCopy(obj)
		if err != nil {
			return false, err
		}
		err = util.AddObjectLabelsWithFlags(objCopy.(runtime.Object), labels, util.ErrorOnExistingDstKey)
		if err != nil {
			return true, nil
		}
	}
	return false, nil
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
	if info.Mapping.GroupVersionKind.GroupKind() == buildapi.Kind("BuildConfig") && isInvalidTriggerError(err) {
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

func handleRunError(err error, baseName, commandName, commandPath string) error {
	if err == nil {
		return nil
	}
	errs := []error{err}
	if agg, ok := err.(errors.Aggregate); ok {
		errs = agg.Errors()
	}
	groups := errorGroups{}
	for _, err := range errs {
		transformError(err, baseName, commandName, commandPath, groups)
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

type errorGroup struct {
	errs       []error
	suggestion string
}
type errorGroups map[string]errorGroup

func (g errorGroups) Add(group string, suggestion string, err error, errs ...error) {
	all := g[group]
	all.errs = append(all.errs, errs...)
	all.errs = append(all.errs, err)
	all.suggestion = suggestion
	g[group] = all
}

func transformError(err error, baseName, commandName, commandPath string, groups errorGroups) {
	switch t := err.(type) {
	case newcmd.ErrRequiresExplicitAccess:
		if t.Input.Token != nil && t.Input.Token.ServiceAccount {
			groups.Add(
				"explicit-access-installer",
				heredoc.Doc(`
					WARNING: This will allow the pod to create and manage resources within your namespace -
					ensure you trust the image with those permissions before you continue.

					You can see more information about the image by adding the --dry-run flag.
					If you trust the provided image, include the flag --grant-install-rights.`,
				),
				fmt.Errorf("installing %q requires an 'installer' service account with project editor access", t.Match.Value),
			)
		} else {
			groups.Add(
				"explicit-access-you",
				heredoc.Doc(`
					WARNING: This will allow the pod to act as you across the entire cluster - ensure you
					trust the image with those permissions before you continue.

					You can see more information about the image by adding the --dry-run flag.
					If you trust the provided image, include the flag --grant-install-rights.`,
				),
				fmt.Errorf("installing %q requires that you grant the image access to run with your credentials", t.Match.Value),
			)
		}
		return
	case newapp.ErrNoMatch:
		groups.Add(
			"no-matches",
			heredoc.Docf(`
				The '%[1]s' command will match arguments to the following types:

				  1. Images tagged into image streams in the current project or the 'openshift' project
				     - if you don't specify a tag, we'll add ':latest'
				  2. Images in the Docker Hub, on remote registries, or on the local Docker engine
				  3. Templates in the current project or the 'openshift' project
				  4. Git repository URLs or local paths that point to Git repositories

				--allow-missing-images can be used to point to an image that does not exist yet.

				See '%[1]s -h' for examples.`, commandPath,
			),
			t,
			t.Errs...,
		)
		return
	case newapp.ErrMultipleMatches:
		buf := &bytes.Buffer{}
		for i, match := range t.Matches {

			// If we have more than 5 matches, stop output and recommend searching
			// after the fifth
			if i >= 5 {
				groups.Add(
					"multiple-matches",
					heredoc.Docf(`
						The argument %[1]q could apply to the following Docker images, OpenShift image streams, or templates:

						%[2]sTo view a full list of matches, use '%[3]s %[4]s -S %[1]s'`, t.Value, buf.String(), baseName, commandName,
					),
					t,
					t.Errs...,
				)

				return
			}

			fmt.Fprintf(buf, "* %s\n", match.Description)
			fmt.Fprintf(buf, "  Use %[1]s to specify this image or template\n\n", match.Argument)
		}

		groups.Add(
			"multiple-matches",
			heredoc.Docf(`
					The argument %[1]q could apply to the following Docker images, OpenShift image streams, or templates:

					%[2]s`, t.Value, buf.String(),
			),
			t,
			t.Errs...,
		)
		return
	case newapp.ErrPartialMatch:
		buf := &bytes.Buffer{}
		fmt.Fprintf(buf, "* %s\n", t.Match.Description)
		fmt.Fprintf(buf, "  Use %[1]s to specify this image or template\n\n", t.Match.Argument)

		groups.Add(
			"partial-match",
			heredoc.Docf(`
					The argument %[1]q only partially matched the following Docker image, OpenShift image stream, or template:

					%[2]s`, t.Value, buf.String(),
			),
			t,
			t.Errs...,
		)
		return
	case newapp.ErrNoTagsFound:
		buf := &bytes.Buffer{}
		fmt.Fprintf(buf, "  Use --allow-missing-imagestream-tags to use this image stream\n\n")
		groups.Add(
			"no-tags",
			heredoc.Docf(`
					The image stream %[1]q exists, but it has no tags.

					%[2]s`, t.Match.Name, buf.String(),
			),
			t,
			t.Errs...,
		)
		return
	}
	switch err {
	case errNoTokenAvailable:
		// TODO: improve by allowing token generation
		groups.Add("", "", fmt.Errorf("to install components you must be logged in with an OAuth token (instead of only a certificate)"))
	case newcmd.ErrNoInputs:
		// TODO: suggest things to the user
		groups.Add("", "", usageError(commandPath, newAppNoInput, baseName, commandName))
	default:
		groups.Add("", "", err)
	}
}

func usageError(commandPath, format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s\nSee '%s -h' for help and examples", msg, commandPath)
}

func printHumanReadableQueryResult(r *newcmd.QueryResult, out io.Writer, baseName, commandName string) error {
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
		fmt.Fprintf(out, "Templates (%s %s --template=<template>)\n", baseName, commandName)
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
		fmt.Fprintf(out, "Image streams (%s %s --image-stream=<image-stream> [--code=<source>])\n", baseName, commandName)
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
				tags = strings.Join(set.List(), ", ")
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
		fmt.Fprintf(out, "Docker images (%s %s --docker-image=<docker-image> [--code=<source>])\n", baseName, commandName)
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

type configSecretRetriever struct {
	config *restclient.Config
}

func newConfigSecretRetriever(config *restclient.Config) newapp.SecretAccessor {
	return &configSecretRetriever{config}
}

var errNoTokenAvailable = fmt.Errorf("you are not logged in with a token - unable to provide a secret to the installable component")

func (r *configSecretRetriever) Token() (string, error) {
	if len(r.config.BearerToken) > 0 {
		return r.config.BearerToken, nil
	}
	return "", errNoTokenAvailable
}

func (r *configSecretRetriever) CACert() (string, error) {
	if len(r.config.CAData) > 0 {
		return string(r.config.CAData), nil
	}
	if len(r.config.CAFile) > 0 {
		data, err := ioutil.ReadFile(r.config.CAFile)
		if err != nil {
			return "", fmt.Errorf("unable to read CA cert from config %s: %v", r.config.CAFile, err)
		}
		return string(data), nil
	}
	return "", nil
}

func checkGitInstalled(w io.Writer) {
	if !git.IsGitInstalled() {
		fmt.Fprintf(w, "warning: Cannot find git. Ensure that it is installed and in your path. Git is required to work with git repositories.\n")
	}
}
