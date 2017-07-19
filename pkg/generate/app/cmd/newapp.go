package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"

	dockerfileparser "github.com/docker/docker/builder/dockerfile/parser"
	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/validation"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	ometa "github.com/openshift/origin/pkg/api/meta"
	authapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/client"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/generate"
	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/generate/dockerfile"
	"github.com/openshift/origin/pkg/generate/jenkinsfile"
	"github.com/openshift/origin/pkg/generate/source"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	outil "github.com/openshift/origin/pkg/util"
	dockerfileutil "github.com/openshift/origin/pkg/util/docker/dockerfile"
)

const (
	GeneratedByNamespace = "openshift.io/generated-by"
	GeneratedForJob      = "openshift.io/generated-job"
	GeneratedForJobFor   = "openshift.io/generated-job.for"
	GeneratedByNewApp    = "OpenShiftNewApp"
	GeneratedByNewBuild  = "OpenShiftNewBuild"
)

// GenerationInputs control how new-app creates output
// TODO: split these into finer grained structs
type GenerationInputs struct {
	TemplateParameters []string
	Environment        []string
	BuildEnvironment   []string
	BuildArgs          []string
	Labels             map[string]string

	TemplateParameterFiles []string
	EnvironmentFiles       []string
	BuildEnvironmentFiles  []string

	IgnoreUnknownParameters bool
	InsecureRegistry        bool

	Strategy generate.Strategy

	Name     string
	To       string
	NoOutput bool

	OutputDocker  bool
	Dockerfile    string
	ExpectToBuild bool
	BinaryBuild   bool
	ContextDir    string

	SourceImage     string
	SourceImagePath string

	Secrets []string

	AllowMissingImageStreamTags bool

	Deploy           bool
	AsTestDeployment bool

	AllowGenerationErrors bool
}

// AppConfig contains all the necessary configuration for an application
type AppConfig struct {
	ComponentInputs
	GenerationInputs

	ResolvedComponents *ResolvedComponents

	SkipGeneration bool

	AllowSecretUse              bool
	AllowNonNumericExposedPorts bool
	SecretAccessor              app.SecretAccessor

	AsSearch bool
	AsList   bool
	DryRun   bool

	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer

	KubeClient kclientset.Interface

	Resolvers

	Typer            runtime.ObjectTyper
	Mapper           meta.RESTMapper
	CategoryExpander resource.CategoryExpander
	ClientMapper     resource.ClientMapper

	OSClient        client.Interface
	OriginNamespace string

	ArgumentClassificationErrors []ArgumentClassificationError
}

type ArgumentClassificationError struct {
	Key   string
	Value error
}

type ErrRequiresExplicitAccess struct {
	Match app.ComponentMatch
	Input app.GeneratorInput
}

func (e ErrRequiresExplicitAccess) Error() string {
	return fmt.Sprintf("the component %q is requesting access to run with your security credentials and install components - you must explicitly grant that access to continue", e.Match.String())
}

// ErrNoInputs is returned when no inputs are specified
var ErrNoInputs = errors.New("no inputs provided")

// AppResult contains the results of an application
type AppResult struct {
	List *kapi.List

	Name      string
	HasSource bool
	Namespace string

	GeneratedJobs bool
}

// QueryResult contains the results of a query (search or list)
type QueryResult struct {
	Matches app.ComponentMatches
	List    *kapi.List
}

// NewAppConfig returns a new AppConfig, but you must set your typer, mapper, and clientMapper after the command has been run
// and flags have been parsed.
func NewAppConfig() *AppConfig {
	return &AppConfig{
		Resolvers: Resolvers{
			Detector: app.SourceRepositoryEnumerator{
				Detectors:         source.DefaultDetectors,
				DockerfileTester:  dockerfile.NewTester(),
				JenkinsfileTester: jenkinsfile.NewTester(),
			},
		},
	}
}

func (c *AppConfig) DockerRegistrySearcher() app.Searcher {
	return app.DockerRegistrySearcher{
		Client:        dockerregistry.NewClient(30*time.Second, true),
		AllowInsecure: c.InsecureRegistry,
	}
}

func (c *AppConfig) ensureDockerSearch() {
	if c.DockerSearcher == nil {
		c.DockerSearcher = c.DockerRegistrySearcher()
	}
}

// SetOpenShiftClient sets the passed OpenShift client in the application configuration
func (c *AppConfig) SetOpenShiftClient(osclient client.Interface, OriginNamespace string, dockerclient *docker.Client) {
	c.OSClient = osclient
	c.OriginNamespace = OriginNamespace
	namespaces := []string{OriginNamespace}
	if openshiftNamespace := "openshift"; OriginNamespace != openshiftNamespace {
		namespaces = append(namespaces, openshiftNamespace)
	}
	c.ImageStreamSearcher = app.ImageStreamSearcher{
		Client:            osclient,
		ImageStreamImages: osclient,
		Namespaces:        namespaces,
		AllowMissingTags:  c.AllowMissingImageStreamTags,
	}
	c.ImageStreamByAnnotationSearcher = app.NewImageStreamByAnnotationSearcher(osclient, osclient, namespaces)
	c.TemplateSearcher = app.TemplateSearcher{
		Client: osclient,
		TemplateConfigsNamespacer: osclient,
		Namespaces:                namespaces,
	}
	c.TemplateFileSearcher = &app.TemplateFileSearcher{
		Typer:            c.Typer,
		Mapper:           c.Mapper,
		ClientMapper:     c.ClientMapper,
		CategoryExpander: c.CategoryExpander,
		Namespace:        OriginNamespace,
	}
	// the hierarchy of docker searching is:
	// 1) if we have an openshift client - query docker registries via openshift,
	// if we're unable to query via openshift, query the docker registries directly(fallback),
	// if we don't find a match there and a local docker daemon exists, look in the local registry.
	// 2) if we don't have an openshift client - query the docker registries directly,
	// if we don't find a match there and a local docker daemon exists, look in the local registry.
	c.DockerSearcher = app.DockerClientSearcher{
		Client:             dockerclient,
		Insecure:           c.InsecureRegistry,
		AllowMissingImages: c.AllowMissingImages,
		RegistrySearcher: app.ImageImportSearcher{
			Client:        osclient.ImageStreams(OriginNamespace),
			AllowInsecure: c.InsecureRegistry,
			Fallback:      c.DockerRegistrySearcher(),
		},
	}
}

func (c *AppConfig) tryToAddEnvironmentArguments(s string) bool {
	rc := cmdutil.IsEnvironmentArgument(s)
	if rc {
		glog.V(2).Infof("treating %s as possible environment argument\n", s)
		c.Environment = append(c.Environment, s)
	}
	return rc
}

func (c *AppConfig) tryToAddSourceArguments(s string) bool {
	remote, rerr := app.IsRemoteRepository(s)
	local, derr := app.IsDirectory(s)

	if remote || local {
		glog.V(2).Infof("treating %s as possible source repo\n", s)
		c.SourceRepositories = append(c.SourceRepositories, s)
		return true
	}

	if rerr != nil {
		c.ArgumentClassificationErrors = append(c.ArgumentClassificationErrors, ArgumentClassificationError{
			Key:   fmt.Sprintf("%s as a Git repository URL", s),
			Value: rerr,
		})
	}

	if derr != nil {
		c.ArgumentClassificationErrors = append(c.ArgumentClassificationErrors, ArgumentClassificationError{
			Key:   fmt.Sprintf("%s as a local directory pointing to a Git repository", s),
			Value: derr,
		})
	}

	return false
}

func (c *AppConfig) tryToAddComponentArguments(s string) bool {
	err := app.IsComponentReference(s)
	if err == nil {
		glog.V(2).Infof("treating %s as a component ref\n", s)
		c.Components = append(c.Components, s)
		return true
	}
	c.ArgumentClassificationErrors = append(c.ArgumentClassificationErrors, ArgumentClassificationError{
		Key:   fmt.Sprintf("%s as a template loaded in an accessible project, an imagestream tag, or a docker image reference", s),
		Value: err,
	})

	return false
}

func (c *AppConfig) tryToAddTemplateArguments(s string) bool {
	rc, err := app.IsPossibleTemplateFile(s)
	if rc {
		glog.V(2).Infof("treating %s as possible template file\n", s)
		c.Components = append(c.Components, s)
		return true
	}
	if err != nil {
		c.ArgumentClassificationErrors = append(c.ArgumentClassificationErrors, ArgumentClassificationError{
			Key:   fmt.Sprintf("%s as a template stored in a local file", s),
			Value: err,
		})
	}
	return false
}

// AddArguments converts command line arguments into the appropriate bucket based on what they look like
func (c *AppConfig) AddArguments(args []string) []string {
	unknown := []string{}
	c.ArgumentClassificationErrors = []ArgumentClassificationError{}
	for _, s := range args {
		if len(s) == 0 {
			continue
		}

		switch {
		case c.tryToAddEnvironmentArguments(s):
		case c.tryToAddSourceArguments(s):
		case c.tryToAddComponentArguments(s):
		case c.tryToAddTemplateArguments(s):
		default:
			glog.V(2).Infof("treating %s as unknown\n", s)
			unknown = append(unknown, s)
		}

	}
	return unknown
}

// validateBuilders confirms that all images associated with components that are to be built,
// are builders (or we're using a non-source strategy).
func (c *AppConfig) validateBuilders(components app.ComponentReferences) error {
	if c.Strategy != generate.StrategyUnspecified {
		return nil
	}
	errs := []error{}
	for _, ref := range components {
		input := ref.Input()
		// if we're supposed to build this thing, and the image/imagestream we've matched it to did not come from an explicit CLI argument,
		// and the image/imagestream we matched to is not explicitly an s2i builder, and we're doing a source-type build, warn the user
		// that this probably won't work and force them to declare their intention explicitly.
		if input.ExpectToBuild && input.ResolvedMatch != nil && !app.IsBuilderMatch(input.ResolvedMatch) && input.Uses != nil && input.Uses.GetStrategy() == generate.StrategySource {
			errs = append(errs, fmt.Errorf("the image match %q for source repository %q does not appear to be a source-to-image builder.\n\n- to attempt to use this image as a source builder, pass \"--strategy=source\"\n- to use it as a base image for a Docker build, pass \"--strategy=docker\"", input.ResolvedMatch.Name, input.Uses))
			continue
		}
	}
	return kutilerrors.NewAggregate(errs)
}

func validateEnforcedName(name string) error {
	// up to 63 characters is nominally possible, however "-1" gets added on the
	// end later for the deployment controller.  Deduct 5 from 63 to at least
	// cover us up to -9999.
	if reasons := validation.ValidateServiceName(name, false); (len(reasons) != 0 || len(name) > 58) && !app.IsParameterizableValue(name) {
		return fmt.Errorf("invalid name: %s. Must be an a lower case alphanumeric (a-z, and 0-9) string with a maximum length of 58 characters, where the first character is a letter (a-z), and the '-' character is allowed anywhere except the first or last character.", name)
	}
	return nil
}

func validateOutputImageReference(ref string) error {
	if _, err := imageapi.ParseDockerImageReference(ref); err != nil {
		return fmt.Errorf("invalid output image reference: %s", ref)
	}
	return nil
}

// buildPipelines converts a set of resolved, valid references into pipelines.
func (c *AppConfig) buildPipelines(components app.ComponentReferences, environment app.Environment, buildEnvironment app.Environment) (app.PipelineGroup, error) {
	pipelines := app.PipelineGroup{}

	buildArgs, err := cmdutil.ParseBuildArg(c.BuildArgs, c.In)
	if err != nil {
		return nil, err
	}

	var DockerStrategyOptions *buildapi.DockerStrategyOptions
	if len(c.BuildArgs) > 0 {
		DockerStrategyOptions = &buildapi.DockerStrategyOptions{
			BuildArgs: buildArgs,
		}
	}

	numDockerBuilds := 0

	pipelineBuilder := app.NewPipelineBuilder(c.Name, buildEnvironment, DockerStrategyOptions, c.OutputDocker).To(c.To)
	for _, group := range components.Group() {
		glog.V(4).Infof("found group: %v", group)
		common := app.PipelineGroup{}
		for _, ref := range group {
			refInput := ref.Input()
			from := refInput.String()
			var pipeline *app.Pipeline

			switch {
			case refInput.ExpectToBuild:
				glog.V(4).Infof("will add %q secrets into a build for a source build of %q", strings.Join(c.Secrets, ","), refInput.Uses)

				if err := refInput.Uses.AddBuildSecrets(c.Secrets); err != nil {
					return nil, fmt.Errorf("unable to add build secrets %q: %v", strings.Join(c.Secrets, ","), err)
				}

				if refInput.Uses.GetStrategy() == generate.StrategyDocker {
					numDockerBuilds++
				}

				var (
					image *app.ImageRef
					err   error
				)
				if refInput.ResolvedMatch != nil {
					inputImage, err := app.InputImageFromMatch(refInput.ResolvedMatch)
					if err != nil {
						return nil, fmt.Errorf("can't build %q: %v", from, err)
					}
					if !inputImage.AsImageStream && from != "scratch" && (refInput.Uses == nil || refInput.Uses.GetStrategy() != generate.StrategyPipeline) {
						msg := "Could not find an image stream match for %q. Make sure that a Docker image with that tag is available on the node for the build to succeed."
						glog.Warningf(msg, from)
					}
					image = inputImage
				}

				glog.V(4).Infof("will use %q as the base image for a source build of %q", ref, refInput.Uses)
				if pipeline, err = pipelineBuilder.NewBuildPipeline(from, image, refInput.Uses, c.BinaryBuild); err != nil {
					return nil, fmt.Errorf("can't build %q: %v", refInput.Uses, err)
				}
			default:
				inputImage, err := app.InputImageFromMatch(refInput.ResolvedMatch)
				if err != nil {
					return nil, fmt.Errorf("can't include %q: %v", from, err)
				}
				if !inputImage.AsImageStream {
					msg := "Could not find an image stream match for %q. Make sure that a Docker image with that tag is available on the node for the deployment to succeed."
					glog.Warningf(msg, from)
				}

				glog.V(4).Infof("will include %q", ref)
				if pipeline, err = pipelineBuilder.NewImagePipeline(from, inputImage); err != nil {
					return nil, fmt.Errorf("can't include %q: %v", refInput, err)
				}
			}
			if c.Deploy {
				if err := pipeline.NeedsDeployment(environment, c.Labels, c.AsTestDeployment); err != nil {
					return nil, fmt.Errorf("can't set up a deployment for %q: %v", refInput, err)
				}
			}
			if c.NoOutput {
				pipeline.Build.Output = nil
			}
			if refInput.Uses != nil && refInput.Uses.GetStrategy() == generate.StrategyPipeline {
				pipeline.Build.Output = nil
				pipeline.Deployment = nil
				pipeline.Image = nil
				pipeline.InputImage = nil
			}
			common = append(common, pipeline)
			if err := common.Reduce(); err != nil {
				return nil, fmt.Errorf("can't create a pipeline from %s: %v", common, err)
			}
			describeBuildPipelineWithImage(c.Out, ref, pipeline, c.OriginNamespace)
		}
		pipelines = append(pipelines, common...)
	}

	if len(c.BuildArgs) > 0 {
		if numDockerBuilds == 0 {
			return nil, fmt.Errorf("Cannot use '--build-arg' without a Docker build")
		}
		if numDockerBuilds > 1 {
			fmt.Fprintf(c.ErrOut, "--> WARNING: Applying --build-arg to multiple Docker builds.\n")
		}
	}

	return pipelines, nil
}

// buildTemplates converts a set of resolved, valid references into references to template objects.
func (c *AppConfig) buildTemplates(components app.ComponentReferences, parameters app.Environment, environment app.Environment, buildEnvironment app.Environment) (string, []runtime.Object, error) {
	objects := []runtime.Object{}
	name := ""
	for _, ref := range components {
		tpl := ref.Input().ResolvedMatch.Template

		glog.V(4).Infof("processing template %s/%s", c.OriginNamespace, tpl.Name)
		if len(c.ContextDir) > 0 {
			return "", nil, fmt.Errorf("--context-dir is not supported when using a template")
		}
		result, err := TransformTemplate(tpl, c.OSClient, c.OriginNamespace, parameters, c.IgnoreUnknownParameters)
		if err != nil {
			return name, nil, err
		}
		if len(name) == 0 {
			name = tpl.Name
		}
		objects = append(objects, result.Objects...)
		if len(result.Objects) > 0 {
			// if environment variables were passed in, let's apply the environment variables
			// to every pod template object
			for i := range result.Objects {
				switch result.Objects[i].(type) {
				case *buildapi.BuildConfig:
					buildEnv := buildutil.GetBuildConfigEnv(result.Objects[i].(*buildapi.BuildConfig))
					buildEnv = app.JoinEnvironment(buildEnv, buildEnvironment.List())
					buildutil.SetBuildConfigEnv(result.Objects[i].(*buildapi.BuildConfig), buildEnv)
				}
				podSpec, _, err := ometa.GetPodSpec(result.Objects[i])
				if err == nil {
					for ii := range podSpec.Containers {
						if podSpec.Containers[ii].Env != nil {
							podSpec.Containers[ii].Env = app.JoinEnvironment(environment.List(), podSpec.Containers[ii].Env)
						} else {
							podSpec.Containers[ii].Env = environment.List()
						}
					}
				}
			}
		}

		DescribeGeneratedTemplate(c.Out, ref.Input().String(), result, c.OriginNamespace)
	}
	return name, objects, nil
}

// fakeSecretAccessor is used during dry runs of installation
type fakeSecretAccessor struct {
	token string
}

func (a *fakeSecretAccessor) Token() (string, error) {
	return a.token, nil
}
func (a *fakeSecretAccessor) CACert() (string, error) {
	return "", nil
}

// installComponents attempts to create pods to run installable images identified by the user. If an image
// is installable, we check whether it requires access to the user token. If so, the caller must have
// explicitly granted that access (because the token may be the user's).
func (c *AppConfig) installComponents(components app.ComponentReferences, env app.Environment) ([]runtime.Object, string, error) {
	if c.SkipGeneration {
		return nil, "", nil
	}

	jobs := components.InstallableComponentRefs()
	switch {
	case len(jobs) > 1:
		return nil, "", fmt.Errorf("only one installable component may be provided: %s", jobs.HumanString(", "))
	case len(jobs) == 0:
		return nil, "", nil
	}

	job := jobs[0]
	if len(components) > 1 {
		return nil, "", fmt.Errorf("%q is installable and may not be specified with other components", job.Input().Value)
	}
	input := job.Input()

	imageRef, err := app.InputImageFromMatch(input.ResolvedMatch)
	if err != nil {
		return nil, "", fmt.Errorf("can't include %q: %v", input, err)
	}
	glog.V(4).Infof("Resolved match for installer %#v", input.ResolvedMatch)

	imageRef.AsImageStream = false
	imageRef.AsResolvedImage = true
	imageRef.Env = env

	name := c.Name
	if len(name) == 0 {
		var ok bool
		name, ok = imageRef.SuggestName()
		if !ok {
			return nil, "", errors.New("can't suggest a valid name, please specify a name with --name")
		}
	}
	imageRef.ObjectName = name
	glog.V(4).Infof("Proposed installable image %#v", imageRef)

	secretAccessor := c.SecretAccessor
	generatorInput := input.ResolvedMatch.GeneratorInput
	token := generatorInput.Token
	if token != nil && !c.AllowSecretUse || secretAccessor == nil {
		if !c.DryRun {
			return nil, "", ErrRequiresExplicitAccess{Match: *input.ResolvedMatch, Input: generatorInput}
		}
		secretAccessor = &fakeSecretAccessor{token: "FAKE_TOKEN"}
	}

	objects := []runtime.Object{}

	serviceAccountName := "installer"
	if token != nil && token.ServiceAccount {
		if _, err := c.KubeClient.Core().ServiceAccounts(c.OriginNamespace).Get(serviceAccountName, metav1.GetOptions{}); err != nil {
			if kerrors.IsNotFound(err) {
				objects = append(objects,
					// create a new service account
					&kapi.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName}},
					// grant the service account the edit role on the project (TODO: installer)
					&authapi.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{Name: "installer-role-binding"},
						Subjects:   []kapi.ObjectReference{{Kind: "ServiceAccount", Name: serviceAccountName}},
						RoleRef:    kapi.ObjectReference{Name: "edit"},
					},
				)
			}
		}
	}

	pod, secret, err := imageRef.InstallablePod(generatorInput, secretAccessor, serviceAccountName)
	if err != nil {
		return nil, "", err
	}
	objects = append(objects, pod)
	if secret != nil {
		objects = append(objects, secret)
	}
	for i := range objects {
		outil.AddObjectAnnotations(objects[i], map[string]string{
			GeneratedForJob:    "true",
			GeneratedForJobFor: input.String(),
		})
	}

	describeGeneratedJob(c.Out, job, pod, secret, c.OriginNamespace)

	return objects, name, nil
}

// RunQuery executes the provided config and returns the result of the resolution.
func (c *AppConfig) RunQuery() (*QueryResult, error) {
	environment, buildEnvironment, parameters, err := c.validate()
	if err != nil {
		return nil, err
	}
	// TODO: I don't belong here
	c.ensureDockerSearch()

	if c.AsList {
		if c.AsSearch {
			return nil, errors.New("--list and --search can't be used together")
		}
		if c.HasArguments() {
			return nil, errors.New("--list can't be used with arguments")
		}
		c.Components = append(c.Components, "*")
	}

	b := &app.ReferenceBuilder{}
	if err := AddComponentInputsToRefBuilder(b, &c.Resolvers, &c.ComponentInputs, &c.GenerationInputs); err != nil {
		return nil, err
	}
	components, repositories, errs := b.Result()
	if len(errs) > 0 {
		return nil, kutilerrors.NewAggregate(errs)
	}

	if len(components) == 0 && !c.AsList {
		return nil, ErrNoInputs
	}

	if len(repositories) > 0 {
		errs = append(errs, errors.New("--search can't be used with source code"))
	}
	if len(environment) > 0 {
		errs = append(errs, errors.New("--search can't be used with --env"))
	}
	if len(buildEnvironment) > 0 {
		errs = append(errs, errors.New("--search can't be used with --build-env"))
	}
	if len(parameters) > 0 {
		errs = append(errs, errors.New("--search can't be used with --param"))
	}
	if len(errs) > 0 {
		return nil, kutilerrors.NewAggregate(errs)
	}

	if err := components.Search(); err != nil {
		return nil, err
	}

	glog.V(4).Infof("Code %v", repositories)
	glog.V(4).Infof("Components %v", components)

	matches := app.ComponentMatches{}
	objects := app.Objects{}
	for _, ref := range components {
		for _, match := range ref.Input().SearchMatches {
			matches = append(matches, match)
			if match.IsTemplate() {
				objects = append(objects, match.Template)
			} else if match.IsImage() {
				if match.ImageStream != nil {
					objects = append(objects, match.ImageStream)
				}
				if match.Image != nil {
					objects = append(objects, match.Image)
				}
			}
		}
	}
	return &QueryResult{
		Matches: matches,
		List:    &kapi.List{Items: objects},
	}, nil
}

func (c *AppConfig) validate() (app.Environment, app.Environment, app.Environment, error) {
	env, err := app.ParseAndCombineEnvironment(c.Environment, c.EnvironmentFiles, c.In, func(key, file string) error {
		if file == "" {
			fmt.Fprintf(c.ErrOut, "warning: Environment variable %q was overwritten\n", key)
		} else {
			fmt.Fprintf(c.ErrOut, "warning: Environment variable %q already defined, ignoring value from file %q\n", key, file)
		}
		return nil
	})
	if err != nil {
		return nil, nil, nil, err
	}
	buildEnv, err := app.ParseAndCombineEnvironment(c.BuildEnvironment, c.BuildEnvironmentFiles, c.In, func(key, file string) error {
		if file == "" {
			fmt.Fprintf(c.ErrOut, "warning: Build Environment variable %q was overwritten\n", key)
		} else {
			fmt.Fprintf(c.ErrOut, "warning: Build Environment variable %q already defined, ignoring value from file %q\n", key, file)
		}
		return nil
	})
	if err != nil {
		return nil, nil, nil, err
	}
	params, err := app.ParseAndCombineEnvironment(c.TemplateParameters, c.TemplateParameterFiles, c.In, func(key, file string) error {
		if file == "" {
			fmt.Fprintf(c.ErrOut, "warning: Template parameter %q was overwritten\n", key)
		} else {
			fmt.Fprintf(c.ErrOut, "warning: Template parameter %q already defined, ignoring value from file %q\n", key, file)
		}
		return nil
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return env, buildEnv, params, nil
}

// Run executes the provided config to generate objects.
func (c *AppConfig) Run() (*AppResult, error) {
	env, buildenv, parameters, err := c.validate()

	if err != nil {
		return nil, err
	}
	// TODO: I don't belong here
	c.ensureDockerSearch()

	resolved, err := Resolve(c)
	if err != nil {
		return nil, err
	}

	repositories := resolved.Repositories
	components := resolved.Components

	if len(repositories) == 0 && len(components) == 0 {
		return nil, ErrNoInputs
	}

	if err := c.validateBuilders(components); err != nil {
		return nil, err
	}

	if len(c.Name) > 0 {
		if err := validateEnforcedName(c.Name); err != nil {
			return nil, err
		}
	}

	if err := optionallyValidateExposedPorts(c, repositories); err != nil {
		return nil, err
	}

	if len(c.To) > 0 {
		if err := validateOutputImageReference(c.To); err != nil {
			return nil, err
		}
	}

	if len(components.ImageComponentRefs().Group()) > 1 && len(c.Name) > 0 {
		return nil, errors.New("only one component or source repository can be used when specifying a name")
	}
	if len(components.UseSource()) > 1 && len(c.To) > 0 {
		return nil, errors.New("only one component with source can be used when specifying an output image reference")
	}

	// identify if there are installable components in the input provided by the user
	installables, name, err := c.installComponents(components, env)
	if err != nil {
		return nil, err
	}
	if len(installables) > 0 {
		return &AppResult{
			List:      &kapi.List{Items: installables},
			Name:      name,
			Namespace: c.OriginNamespace,

			GeneratedJobs: true,
		}, nil
	}

	pipelines, err := c.buildPipelines(components.ImageComponentRefs(), env, buildenv)
	if err != nil {
		return nil, err
	}

	acceptors := app.Acceptors{app.NewAcceptUnique(c.Typer), app.AcceptNew}
	objects := app.Objects{}
	accept := app.NewAcceptFirst()
	for _, p := range pipelines {
		accepted, err := p.Objects(accept, acceptors)
		if err != nil {
			return nil, fmt.Errorf("can't setup %q: %v", p.From, err)
		}
		objects = append(objects, accepted...)
	}

	objects = app.AddServices(objects, false)

	templateName, templateObjects, err := c.buildTemplates(components.TemplateComponentRefs(), parameters, env, buildenv)
	if err != nil {
		return nil, err
	}

	// check for circular reference specifically from the template objects and print warnings if they exist
	err = c.checkCircularReferences(templateObjects)
	if err != nil {
		if err, ok := err.(app.CircularOutputReferenceError); ok {
			// templates only apply to `oc new-app`
			addOn := ""
			if len(c.Name) == 0 {
				addOn = ", override artifact names with --name"
			}
			fmt.Fprintf(c.ErrOut, "--> WARNING: %v\n%s", err, addOn)
		} else {
			return nil, err
		}
	}
	// check for circular reference specifically from the newly generated objects, handling new-app vs. new-build nuances as needed
	err = c.checkCircularReferences(objects)
	if err != nil {
		if err, ok := err.(app.CircularOutputReferenceError); ok {
			if c.ExpectToBuild {
				// circular reference handling for `oc new-build`.
				if len(c.To) == 0 {
					// Output reference was generated, return error.
					return nil, fmt.Errorf("%v, set a different tag with --to", err)
				}
				// Output reference was explicitly provided, print warning.
				fmt.Fprintf(c.ErrOut, "--> WARNING: %v\n", err)
			} else {
				// circular reference handling for `oc new-app`
				if len(c.Name) == 0 {
					return nil, fmt.Errorf("%v, override artifact names with --name", err)
				}
				// Output reference was explicitly provided, print warning.
				fmt.Fprintf(c.ErrOut, "--> WARNING: %v\n", err)
			}
		} else {
			return nil, err
		}
	}

	objects = append(objects, templateObjects...)

	name = c.Name
	if len(name) == 0 {
		name = templateName
	}
	if len(name) == 0 {
		for _, pipeline := range pipelines {
			if pipeline.Deployment != nil {
				name = pipeline.Deployment.Name
				break
			}
		}
	}
	if len(name) == 0 {
		for _, obj := range objects {
			if bc, ok := obj.(*buildapi.BuildConfig); ok {
				name = bc.Name
				break
			}
		}
	}

	return &AppResult{
		List:      &kapi.List{Items: objects},
		Name:      name,
		HasSource: len(repositories) != 0,
		Namespace: c.OriginNamespace,
	}, nil
}

// followRefToDockerImage follows a buildconfig...To/From reference until it
// terminates in docker image information. This can include dereferencing chains
// of ImageStreamTag references that already exist or which are being created.
// ref is the reference to To/From to follow. If ref is an ImageStreamTag
// that is following another ImageStreamTag, isContext should be set to the
// parent IS. Finally, objects is the list of objects that new-app is creating
// to support the buildconfig. It returns a reference to a terminal DockerImage
// or nil if one could not be determined (a valid, non-error outcome). err
// is only used to indicate that the follow encountered a severe error
// (e.g malformed data).
func (c *AppConfig) followRefToDockerImage(ref *kapi.ObjectReference, isContext *imageapi.ImageStream, objects app.Objects) (*kapi.ObjectReference, error) {

	if ref == nil {
		return nil, errors.New("Unable to follow nil")
	}

	if ref.Kind == "DockerImage" {
		// Make a shallow copy so we don't modify the ObjectReference properties that
		// new-app/build created.
		copy := *ref
		// Namespace should not matter here. The DockerImage URL will include project
		// information if it is relevant.
		copy.Namespace = ""

		// DockerImage names may or may not have a tag suffix. Add :latest if there
		// is no tag so that string comparison will behave as expected.
		if !strings.Contains(copy.Name, ":") {
			copy.Name += ":" + imageapi.DefaultImageTag
		}
		return &copy, nil
	}

	if ref.Kind == "ImageStreamImage" {
		// even if the associated tag for this ImageStreamImage matches a output ImageStreamTag, when the image
		// is built it will have a new sha ... you are essentially using a single/unique older version of a image as the base
		// to build future images;  all this means we can leave the ref name as is and return with no error
		// also do shallow copy like above
		copy := *ref
		return &copy, nil
	}

	if ref.Kind != "ImageStreamTag" {
		return nil, fmt.Errorf("Unable to follow reference type: %q", ref.Kind)
	}

	isNS := ref.Namespace
	if len(isNS) == 0 {
		isNS = c.OriginNamespace
	}

	// Otherwise, we are tracing an IST reference
	isName, isTag, ok := imageapi.SplitImageStreamTag(ref.Name)
	if !ok {
		if isContext == nil {
			return nil, fmt.Errorf("Unable to parse ImageStreamTag reference: %q", ref.Name)
		}
		// Otherwise, we are following a tag that references another tag in the same ImageStream.
		isName = isContext.Name
		isTag = ref.Name
	} else {
		// The imagestream is usually being created alongside the buildconfig
		// when new-build is being used, so scan objects being created for it.
		for _, check := range objects {
			if is2, ok := check.(*imageapi.ImageStream); ok {
				if is2.Name == isName {
					isContext = is2
					break
				}
			}
		}

		if isContext == nil {
			var err error
			isContext, err = c.OSClient.ImageStreams(isNS).Get(isName, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("Unable to check for circular build input/outputs: %v", err)
			}
		}
	}

	// Dereference ImageStreamTag to see what it is pointing to
	target := isContext.Spec.Tags[isTag].From

	if target == nil {
		if isContext.Spec.DockerImageRepository == "" {
			// Otherwise, this appears to be a new IS, created by new-app, with very little information
			// populated. We cannot resolve a DockerImage.
			return nil, nil
		}
		// Legacy InputStream without tag support? Spoof what we need.
		imageName := isContext.Spec.DockerImageRepository + ":" + isTag
		return &kapi.ObjectReference{
			Kind: "DockerImage",
			Name: imageName,
		}, nil
	}

	return c.followRefToDockerImage(target, isContext, objects)
}

// checkCircularReferences ensures there are no builds that can trigger themselves
// due to an imagechangetrigger that matches the output destination of the image.
// objects is a list of api objects produced by new-app.
func (c *AppConfig) checkCircularReferences(objects app.Objects) error {
	for i, obj := range objects {

		if glog.V(5) {
			json, _ := json.MarshalIndent(obj, "", "\t")
			glog.Infof("\n\nCycle check input object %v:\n%v\n", i, string(json))
		}

		if bc, ok := obj.(*buildapi.BuildConfig); ok {
			input := buildutil.GetInputReference(bc.Spec.Strategy)
			output := bc.Spec.Output.To

			if output == nil || input == nil {
				return nil
			}

			dockerInput, err := c.followRefToDockerImage(input, nil, objects)
			if err != nil {
				glog.Warningf("Unable to check for circular build input: %v", err)
				return nil
			}
			glog.V(5).Infof("Post follow input:\n%#v\n", dockerInput)

			dockerOutput, err := c.followRefToDockerImage(output, nil, objects)
			if err != nil {
				glog.Warningf("Unable to check for circular build output: %v", err)
				return nil
			}
			glog.V(5).Infof("Post follow:\n%#v\n", dockerOutput)

			if dockerInput != nil && dockerOutput != nil {
				if reflect.DeepEqual(dockerInput, dockerOutput) {
					return app.CircularOutputReferenceError{Reference: fmt.Sprintf("%s", dockerInput.Name)}
				}
			}

			// If it is not possible to follow input and output out to DockerImages,
			// it is likely they are referencing newly created ImageStreams. Just
			// make sure they are not the same image stream.
			inCopy := *input
			outCopy := *output
			for _, ref := range []*kapi.ObjectReference{&inCopy, &outCopy} {
				// Some code paths add namespace and others don't. Make things
				// consistent.
				if len(ref.Namespace) == 0 {
					ref.Namespace = c.OriginNamespace
				}
			}

			if reflect.DeepEqual(inCopy, outCopy) {
				return app.CircularOutputReferenceError{Reference: fmt.Sprintf("%s/%s", inCopy.Namespace, inCopy.Name)}
			}
		}
	}
	return nil
}

func (c *AppConfig) Querying() bool {
	return c.AsList || c.AsSearch
}

func (c *AppConfig) HasArguments() bool {
	return len(c.Components) > 0 ||
		len(c.ImageStreams) > 0 ||
		len(c.DockerImages) > 0 ||
		len(c.Templates) > 0 ||
		len(c.TemplateFiles) > 0
}

func optionallyValidateExposedPorts(config *AppConfig, repositories app.SourceRepositories) error {
	if config.AllowNonNumericExposedPorts {
		return nil
	}

	if config.Strategy != generate.StrategyUnspecified && config.Strategy != generate.StrategyDocker {
		return nil
	}

	for _, repo := range repositories {
		if repoInfo := repo.Info(); repoInfo != nil && repoInfo.Dockerfile != nil {
			node := repoInfo.Dockerfile.AST()
			if err := exposedPortsAreNumeric(node); err != nil {
				return fmt.Errorf("the Dockerfile has an invalid EXPOSE instruction: %v", err)
			}
		}
	}

	return nil
}

func exposedPortsAreNumeric(node *dockerfileparser.Node) error {
	for _, port := range dockerfileutil.LastExposedPorts(node) {
		if _, err := strconv.ParseInt(port, 10, 32); err != nil {
			return fmt.Errorf("could not parse %q: must be numeric", port)
		}
	}
	return nil
}
