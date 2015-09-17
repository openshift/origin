package cmd

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/types"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/errors"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/generate/dockerfile"
	"github.com/openshift/origin/pkg/generate/source"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/template"
	"github.com/openshift/origin/pkg/util/namer"
)

const (
	GeneratedByNamespace = "openshift.io/generated-by"
	GeneratedByNewApp    = "OpenShiftNewApp"
	GeneratedByNewBuild  = "OpenShiftNewBuild"
)

// AppConfig contains all the necessary configuration for an application
type AppConfig struct {
	SourceRepositories util.StringList
	ContextDir         string

	Components    util.StringList
	ImageStreams  util.StringList
	DockerImages  util.StringList
	Templates     util.StringList
	TemplateFiles util.StringList

	TemplateParameters util.StringList
	Groups             util.StringList
	Environment        util.StringList
	Labels             map[string]string

	AddEnvironmentToBuild bool

	Dockerfile string

	Name             string
	Strategy         string
	InsecureRegistry bool
	OutputDocker     bool

	AsSearch bool
	AsList   bool

	refBuilder *app.ReferenceBuilder

	dockerSearcher                  app.Searcher
	imageStreamSearcher             app.Searcher
	imageStreamByAnnotationSearcher app.Searcher
	templateSearcher                app.Searcher
	templateFileSearcher            app.Searcher

	detector app.Detector

	typer        runtime.ObjectTyper
	mapper       meta.RESTMapper
	clientMapper resource.ClientMapper

	osclient        client.Interface
	originNamespace string
}

// UsageError is an interface for printing usage errors
type UsageError interface {
	UsageError(commandName string) string
}

// TODO: replace with upstream converting [1]error to error
type errlist interface {
	Errors() []error
}

// NewAppConfig returns a new AppConfig
func NewAppConfig(typer runtime.ObjectTyper, mapper meta.RESTMapper, clientMapper resource.ClientMapper) *AppConfig {
	dockerSearcher := app.DockerRegistrySearcher{
		Client: dockerregistry.NewClient(),
	}
	return &AppConfig{
		detector: app.SourceRepositoryEnumerator{
			Detectors: source.DefaultDetectors,
			Tester:    dockerfile.NewTester(),
		},
		dockerSearcher: dockerSearcher,
		typer:          typer,
		mapper:         mapper,
		clientMapper:   clientMapper,
		refBuilder:     &app.ReferenceBuilder{},
	}
}

func (c *AppConfig) dockerRegistrySearcher() app.Searcher {
	return app.DockerRegistrySearcher{
		Client:        dockerregistry.NewClient(),
		AllowInsecure: c.InsecureRegistry,
	}
}

func (c *AppConfig) ensureDockerSearcher() {
	if c.dockerSearcher == nil {
		c.dockerSearcher = c.dockerRegistrySearcher()
	}
}

// SetDockerClient sets the passed Docker client in the application configuration
func (c *AppConfig) SetDockerClient(dockerclient *docker.Client) {
	c.dockerSearcher = app.DockerClientSearcher{
		Client:           dockerclient,
		RegistrySearcher: c.dockerRegistrySearcher(),
		Insecure:         c.InsecureRegistry,
	}
}

// SetOpenShiftClient sets the passed OpenShift client in the application configuration
func (c *AppConfig) SetOpenShiftClient(osclient client.Interface, originNamespace string) {
	c.osclient = osclient
	c.originNamespace = originNamespace
	namespaces := []string{originNamespace}
	if openshiftNamespace := "openshift"; originNamespace != openshiftNamespace {
		namespaces = append(namespaces, openshiftNamespace)
	}
	c.imageStreamSearcher = app.ImageStreamSearcher{
		Client:            osclient,
		ImageStreamImages: osclient,
		Namespaces:        namespaces,
	}
	c.imageStreamByAnnotationSearcher = app.NewImageStreamByAnnotationSearcher(osclient, osclient, namespaces)
	c.templateSearcher = app.TemplateSearcher{
		Client: osclient,
		TemplateConfigsNamespacer: osclient,
		Namespaces:                namespaces,
	}
	c.templateFileSearcher = &app.TemplateFileSearcher{
		Typer:        c.typer,
		Mapper:       c.mapper,
		ClientMapper: c.clientMapper,
		Namespace:    originNamespace,
	}
}

// AddArguments converts command line arguments into the appropriate bucket based on what they look like
func (c *AppConfig) AddArguments(args []string) []string {
	unknown := []string{}
	for _, s := range args {
		switch {
		case cmdutil.IsEnvironmentArgument(s):
			c.Environment = append(c.Environment, s)
		case app.IsPossibleSourceRepository(s):
			c.SourceRepositories = append(c.SourceRepositories, s)
		case app.IsComponentReference(s):
			c.Components = append(c.Components, s)
		case app.IsPossibleTemplateFile(s):
			c.Components = append(c.Components, s)
		default:
			if len(s) == 0 {
				break
			}
			unknown = append(unknown, s)
		}
	}
	return unknown
}

// individualSourceRepositories collects the list of SourceRepositories specified in the
// command line that are not associated with a builder using a '~'.
func (c *AppConfig) individualSourceRepositories() (app.SourceRepositories, error) {
	first := true
	for _, s := range c.SourceRepositories {
		if repo, ok := c.refBuilder.AddSourceRepository(s); ok && first {
			repo.SetContextDir(c.ContextDir)
			first = false
		}
	}
	if len(c.Dockerfile) > 0 {
		switch {
		case c.Strategy == "docker", len(c.Strategy) == 0:
		default:
			return nil, fmt.Errorf("when directly referencing a Dockerfile, the strategy must must be 'docker'")
		}
		repo, err := app.NewSourceRepositoryForDockerfile(c.Dockerfile)
		if err != nil {
			return nil, fmt.Errorf("provided Dockerfile is not valid: %v", err)
		}
		c.refBuilder.AddExistingSourceRepository(repo)
	}
	_, repos, errs := c.refBuilder.Result()
	return repos, errors.NewAggregate(errs)
}

// set up the components to be used by the reference builder
func (c *AppConfig) addReferenceBuilderComponents(b *app.ReferenceBuilder) {
	b.AddComponents(c.DockerImages, func(input *app.ComponentInput) app.ComponentReference {
		input.Argument = fmt.Sprintf("--docker-image=%q", input.From)
		input.Searcher = c.dockerSearcher
		if c.dockerSearcher != nil {
			input.Resolver = app.UniqueExactOrInexactMatchResolver{Searcher: c.dockerSearcher}
		}
		return input
	})
	b.AddComponents(c.ImageStreams, func(input *app.ComponentInput) app.ComponentReference {
		input.Argument = fmt.Sprintf("--image-stream=%q", input.From)
		input.Searcher = c.imageStreamSearcher
		if c.imageStreamSearcher != nil {
			input.Resolver = app.FirstMatchResolver{Searcher: c.imageStreamSearcher}
		}
		return input
	})
	b.AddComponents(c.Templates, func(input *app.ComponentInput) app.ComponentReference {
		input.Argument = fmt.Sprintf("--template=%q", input.From)
		input.Searcher = c.templateSearcher
		if c.templateSearcher != nil {
			input.Resolver = app.HighestScoreResolver{Searcher: c.templateSearcher}
		}
		return input
	})
	b.AddComponents(c.TemplateFiles, func(input *app.ComponentInput) app.ComponentReference {
		input.Argument = fmt.Sprintf("--file=%q", input.From)
		input.Searcher = c.templateFileSearcher
		if c.templateFileSearcher != nil {
			input.Resolver = app.FirstMatchResolver{Searcher: c.templateFileSearcher}
		}
		return input
	})
	b.AddComponents(c.Components, func(input *app.ComponentInput) app.ComponentReference {
		resolver := app.PerfectMatchWeightedResolver{}
		searcher := app.MultiWeightedSearcher{}
		if c.imageStreamSearcher != nil {
			resolver = append(resolver, app.WeightedResolver{Searcher: c.imageStreamSearcher, Weight: 0.0})
			searcher = append(searcher, app.WeightedSearcher{Searcher: c.imageStreamSearcher, Weight: 0.0})
		}
		if c.templateSearcher != nil {
			resolver = append(resolver, app.WeightedResolver{Searcher: c.templateSearcher, Weight: 0.0})
			searcher = append(searcher, app.WeightedSearcher{Searcher: c.templateSearcher, Weight: 0.0})
		}
		if c.templateFileSearcher != nil {
			resolver = append(resolver, app.WeightedResolver{Searcher: c.templateFileSearcher, Weight: 0.0})
		}
		if c.dockerSearcher != nil {
			resolver = append(resolver, app.WeightedResolver{Searcher: c.dockerSearcher, Weight: 2.0})
			searcher = append(searcher, app.WeightedSearcher{Searcher: c.dockerSearcher, Weight: 1.0})
		}
		input.Resolver = resolver
		input.Searcher = searcher
		return input
	})
}

// validate converts all of the arguments on the config into references to objects, or returns an error
func (c *AppConfig) validate() (app.ComponentReferences, app.SourceRepositories, cmdutil.Environment, cmdutil.Environment, error) {
	b := c.refBuilder
	c.addReferenceBuilderComponents(b)
	b.AddGroups(c.Groups)
	refs, repos, errs := b.Result()

	if len(c.ContextDir) > 0 && len(repos) > 0 {
		repos[0].SetContextDir(c.ContextDir)
		if len(repos) > 1 {
			glog.Warningf("You have specified more than one source repository and a context directory. "+
				"The context directory will be applied to the first repository: %q", repos[0])
		}
	}

	if len(c.Strategy) != 0 && len(repos) == 0 {
		errs = append(errs, fmt.Errorf("when --strategy is specified you must provide at least one source code location"))
	}

	env, duplicateEnv, envErrs := cmdutil.ParseEnvironmentArguments(c.Environment)
	for _, s := range duplicateEnv {
		glog.V(1).Infof("The environment variable %q was overwritten", s)
	}
	errs = append(errs, envErrs...)

	parms, duplicateParms, parmsErrs := cmdutil.ParseEnvironmentArguments(c.TemplateParameters)
	for _, s := range duplicateParms {
		glog.V(1).Infof("The template parameter %q was overwritten", s)
	}
	errs = append(errs, parmsErrs...)

	return refs, repos, env, parms, errors.NewAggregate(errs)
}

// componentsForRepos creates components for repositories that have not been previously associated by a builder
// these components have already gone through source code detection and have a SourceRepositoryInfo attached to them
func (c *AppConfig) componentsForRepos(repositories app.SourceRepositories) (app.ComponentReferences, error) {
	b := c.refBuilder
	errs := []error{}
	result := app.ComponentReferences{}
	for _, repo := range repositories {
		info := repo.Info()
		switch {
		case info == nil:
			errs = append(errs, fmt.Errorf("source not detected for repository %q", repo))
			continue
		case info.Dockerfile != nil && (len(c.Strategy) == 0 || c.Strategy == "docker"):
			df, err := info.Dockerfile.Dockerfile()
			if err != nil {
				// an unparseable docker file should not terminate the entire create - it may be valid to Docker but not to us
				glog.V(1).Infof("The Dockerfile for the repository %q could not be parsed - no image stream can be created for the FROM", info.Path)
				continue
			}
			dockerFrom, ok := df.GetDirective("FROM")
			if !ok || len(dockerFrom) > 1 {
				errs = append(errs, fmt.Errorf("invalid FROM directive in Dockerfile in repository %q", repo))
			}
			refs := b.AddComponents(dockerFrom, func(input *app.ComponentInput) app.ComponentReference {
				resolver := app.PerfectMatchWeightedResolver{}
				if c.imageStreamSearcher != nil {
					resolver = append(resolver, app.WeightedResolver{Searcher: c.imageStreamSearcher, Weight: 0.0})
				}
				if c.dockerSearcher != nil {
					resolver = append(resolver, app.WeightedResolver{Searcher: c.dockerSearcher, Weight: 1.0})
				}
				resolver = append(resolver, app.WeightedResolver{Searcher: &app.PassThroughDockerSearcher{}, Weight: 2.0})
				input.Resolver = resolver
				input.Use(repo)
				input.ExpectToBuild = true
				repo.UsedBy(input)
				repo.BuildWithDocker()
				return input
			})
			result = append(result, refs...)
		default:
			// TODO: Add support for searching for more than one language if len(info.Types) > 1
			if len(info.Types) == 0 {
				errs = append(errs, fmt.Errorf("no language was detected for repository at %q; please specify a builder image to use with your repository: [builder-image]~%s", repo, repo))

				continue
			}
			refs := b.AddComponents([]string{info.Types[0].Term()}, func(input *app.ComponentInput) app.ComponentReference {
				resolver := app.PerfectMatchWeightedResolver{}
				if c.imageStreamByAnnotationSearcher != nil {
					resolver = append(resolver, app.WeightedResolver{Searcher: c.imageStreamByAnnotationSearcher, Weight: 0.0})
				}
				if c.imageStreamSearcher != nil {
					resolver = append(resolver, app.WeightedResolver{Searcher: c.imageStreamSearcher, Weight: 1.0})
				}
				if c.dockerSearcher != nil {
					resolver = append(resolver, app.WeightedResolver{Searcher: c.dockerSearcher, Weight: 2.0})
				}
				input.Resolver = resolver
				input.ExpectToBuild = true
				input.Use(repo)
				repo.UsedBy(input)
				return input
			})
			result = append(result, refs...)
		}
	}
	return result, errors.NewAggregate(errs)
}

// resolve the references to ensure they are all valid, and identify any images that don't match user input.
func (c *AppConfig) resolve(components app.ComponentReferences) error {
	errs := []error{}
	for _, ref := range components {
		if err := ref.Resolve(); err != nil {
			errs = append(errs, err)
			continue
		}
		switch input := ref.Input(); {
		case !input.ExpectToBuild && input.ResolvedMatch.Builder:
			if c.Strategy != "docker" {
				glog.Infof("Image %q is a builder, so a repository will be expected unless you also specify --strategy=docker", input)
				input.ExpectToBuild = true
			}
		case input.ExpectToBuild && input.ResolvedMatch.IsTemplate():
			// TODO: harder - break the template pieces and check if source code can be attached (look for a build config, build image, etc)
			errs = append(errs, fmt.Errorf("template with source code explicitly attached is not supported - you must either specify the template and source code separately or attach an image to the source code using the '[image]~[code]' form"))
			continue
		case input.ExpectToBuild && !input.ResolvedMatch.Builder && !input.Uses.IsDockerBuild():
			if len(c.Strategy) == 0 {
				errs = append(errs, fmt.Errorf("none of the images that match %q can build source code - check whether this is the image you want to use, then use --strategy=source to build using source or --strategy=docker to treat this as a Docker base image and set up a layered Docker build", ref))
				continue
			}
		}
	}
	return errors.NewAggregate(errs)
}

// searches on all references
func (c *AppConfig) search(components app.ComponentReferences) error {
	errs := []error{}
	for _, ref := range components {
		if err := ref.Search(); err != nil {
			errs = append(errs, err)
			continue
		}
	}
	return errors.NewAggregate(errs)
}

// ensureHasSource ensure every builder component has source code associated with it. It takes a list of component references
// that are builders and have not been associated with source, and a set of source repositories that have not been associated
// with a builder
func (c *AppConfig) ensureHasSource(components app.ComponentReferences, repositories app.SourceRepositories) error {
	if len(components) > 0 {
		switch {
		case len(repositories) > 1:
			if len(components) == 1 {
				component := components[0]
				suggestions := ""

				for _, repo := range repositories {
					suggestions += fmt.Sprintf("%s~%s\n", component, repo)
				}
				return fmt.Errorf("there are multiple code locations provided - use one of the following suggestions to declare which code goes with the image:\n%s", suggestions)
			}
			return fmt.Errorf("the following images require source code: %s\n"+
				" and the following repositories are not used: %s\nUse '[image]~[repo]' to declare which code goes with which image", components, repositories)
		case len(repositories) == 1:
			glog.Infof("Using %q as the source for build", repositories[0])
			for _, component := range components {
				component.Input().Use(repositories[0])
				repositories[0].UsedBy(component)
			}
		default:
			for _, component := range components {
				component.Input().ExpectToBuild = false
			}
		}
	}
	return nil
}

// detectSource runs a code detector on the passed in repositories to obtain a SourceRepositoryInfo
func (c *AppConfig) detectSource(repositories []*app.SourceRepository) error {
	errs := []error{}
	for _, repo := range repositories {
		err := repo.Detect(c.detector)
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}
	return errors.NewAggregate(errs)
}

func (c *AppConfig) validateEnforcedName() error {
	if ok, _ := validation.ValidateServiceName(c.Name, false); !ok {
		return fmt.Errorf("invalid name: %s. Must be an a lower case alphanumeric (a-z, and 0-9) string with a maximum length of 24 characters, where the first character is a letter (a-z), and the '-' character is allowed anywhere except the first or last character.", c.Name)
	}
	return nil
}

func ensureValidUniqueName(names map[string]int, name string) (string, error) {
	// Ensure that name meets length requirements
	if len(name) < 2 {
		return "", fmt.Errorf("invalid name: %s", name)
	}
	if len(name) > util.DNS1123SubdomainMaxLength {
		glog.V(4).Infof("Trimming %s to maximum allowable length (%d)\n", name, util.DNS1123SubdomainMaxLength)
		name = name[:util.DNS1123SubdomainMaxLength]
	}

	// Make all names lowercase
	name = strings.ToLower(name)

	count, existing := names[name]
	if !existing {
		names[name] = 0
		return name, nil
	}
	count++
	names[name] = count
	newName := namer.GetName(name, strconv.Itoa(count), util.DNS1123SubdomainMaxLength)
	return newName, nil
}

// buildPipelines converts a set of resolved, valid references into pipelines.
func (c *AppConfig) buildPipelines(components app.ComponentReferences, environment app.Environment) (app.PipelineGroup, error) {
	pipelines := app.PipelineGroup{}
	names := map[string]int{}
	for _, group := range components.Group() {
		glog.V(2).Infof("found group: %#v", group)
		common := app.PipelineGroup{}
		for _, ref := range group {
			var pipeline *app.Pipeline
			var name string
			if ref.Input().ExpectToBuild {
				glog.V(2).Infof("will use %q as the base image for a source build of %q", ref, ref.Input().Uses)
				input, err := app.InputImageFromMatch(ref.Input().ResolvedMatch)
				if err != nil {
					return nil, fmt.Errorf("can't build %q: %v", ref.Input(), err)
				}
				if !input.AsImageStream {
					glog.Warningf("Could not find an image match for %q. Make sure that a Docker image with that tag is available on the node for the build to succeed.", ref.Input().ResolvedMatch.Value)
				}
				strategy, source, err := app.StrategyAndSourceForRepository(ref.Input().Uses, input)
				if err != nil {
					return nil, fmt.Errorf("can't build %q: %v", ref.Input(), err)
				}
				// Override resource names from the cli
				name = c.Name
				if len(name) == 0 {
					var ok bool
					name, ok = (app.NameSuggestions{source, input}).SuggestName()
					if !ok {
						return nil, fmt.Errorf("can't suggest a valid name, please specify a name with --name")
					}
				}
				name, err = ensureValidUniqueName(names, name)
				source.Name = name
				if err != nil {
					return nil, err
				}

				// Append any exposed ports from Dockerfile to input image
				if ref.Input().Uses.IsDockerBuild() {
					if df, err := ref.Input().Uses.Info().Dockerfile.Dockerfile(); err == nil {
						exposed, ok := df.GetDirective("EXPOSE")
						if ok {
							if input.Info == nil {
								input.Info = &imageapi.DockerImage{
									Config: &imageapi.DockerConfig{},
								}
							}
							input.Info.Config.ExposedPorts = map[string]struct{}{}
							for _, p := range exposed {
								input.Info.Config.ExposedPorts[p] = struct{}{}
							}
						}
					}
				}
				if pipeline, err = app.NewBuildPipeline(ref.Input().String(), input, c.OutputDocker, strategy, c.GetBuildEnvironment(environment), source); err != nil {
					return nil, fmt.Errorf("can't build %q: %v", ref.Input(), err)
				}
			} else {
				glog.V(2).Infof("will include %q", ref)
				input, err := app.InputImageFromMatch(ref.Input().ResolvedMatch)
				if err != nil {
					return nil, fmt.Errorf("can't include %q: %v", ref.Input(), err)
				}
				name = c.Name
				if len(name) == 0 {
					var ok bool
					name, ok = input.SuggestName()
					if !ok {
						return nil, fmt.Errorf("can't suggest a valid name, please specify a name with --name")
					}
				}
				name, err = ensureValidUniqueName(names, name)
				if err != nil {
					return nil, err
				}
				input.ObjectName = name
				if pipeline, err = app.NewImagePipeline(ref.Input().String(), input); err != nil {
					return nil, fmt.Errorf("can't include %q: %v", ref.Input(), err)
				}
			}
			if err := pipeline.NeedsDeployment(environment, c.Labels, name); err != nil {
				return nil, fmt.Errorf("can't set up a deployment for %q: %v", ref.Input(), err)
			}
			common = append(common, pipeline)
		}

		if err := common.Reduce(); err != nil {
			return nil, fmt.Errorf("can't create a pipeline from %s: %v", common, err)
		}
		pipelines = append(pipelines, common...)
	}
	return pipelines, nil
}

// buildTemplates converts a set of resolved, valid references into references to template objects.
func (c *AppConfig) buildTemplates(components app.ComponentReferences, environment app.Environment) ([]runtime.Object, error) {
	objects := []runtime.Object{}

	for _, ref := range components {
		tpl := ref.Input().ResolvedMatch.Template

		glog.V(4).Infof("processing template %s/%s", c.originNamespace, tpl.Name)
		for _, env := range environment.List() {
			// only set environment values that match what's expected by the template.
			if v := template.GetParameterByName(tpl, env.Name); v != nil {
				v.Value = env.Value
				v.Generate = ""
				template.AddParameter(tpl, *v)
			} else {
				return nil, fmt.Errorf("unexpected parameter name %q", env.Name)
			}
		}

		result, err := c.osclient.TemplateConfigs(c.originNamespace).Create(tpl)
		if err != nil {
			return nil, fmt.Errorf("error processing template %s/%s: %v", c.originNamespace, tpl.Name, err)
		}
		errs := runtime.DecodeList(result.Objects, kapi.Scheme)
		if len(errs) > 0 {
			err = errors.NewAggregate(errs)
			return nil, fmt.Errorf("error processing template %s/%s: %v", c.originNamespace, tpl.Name, errs)
		}
		objects = append(objects, result.Objects...)
	}
	return objects, nil
}

// ErrNoInputs is returned when no inputs are specified
var ErrNoInputs = fmt.Errorf("no inputs provided")

// AppResult contains the results of an application
type AppResult struct {
	List *kapi.List

	Name       string
	BuildNames []string
	HasSource  bool
	Namespace  string
}

// QueryResult contains the results of a query (search or list)
type QueryResult struct {
	Matches app.ComponentMatches
	List    *kapi.List
}

// RunAll executes the provided config to generate all objects.
func (c *AppConfig) RunAll(out, errOut io.Writer) (*AppResult, error) {
	return c.run(out, errOut, app.Acceptors{app.NewAcceptUnique(c.typer), app.AcceptNew})
}

// RunBuilds executes the provided config to generate just builds.
func (c *AppConfig) RunBuilds(out, errOut io.Writer) (*AppResult, error) {
	bcAcceptor := app.NewAcceptBuildConfigs(c.typer)
	result, err := c.run(out, errOut, app.Acceptors{bcAcceptor, app.NewAcceptUnique(c.typer), app.AcceptNew})
	if err != nil {
		return nil, err
	}
	return filterImageStreams(result), nil
}

func filterImageStreams(result *AppResult) *AppResult {
	// 1st pass to get images from all BuildConfigs
	imageStreams := map[string]bool{}
	for _, item := range result.List.Items {
		if bc, ok := item.(*buildapi.BuildConfig); ok {
			to := bc.Spec.Output.To
			if to != nil && to.Kind == "ImageStreamTag" {
				imageStreams[makeImageStreamKey(*to)] = true
			}
			switch bc.Spec.Strategy.Type {
			case buildapi.DockerBuildStrategyType:
				from := bc.Spec.Strategy.DockerStrategy.From
				if from != nil && from.Kind == "ImageStreamTag" {
					imageStreams[makeImageStreamKey(*from)] = true
				}
			case buildapi.SourceBuildStrategyType:
				from := bc.Spec.Strategy.SourceStrategy.From
				if from.Kind == "ImageStreamTag" {
					imageStreams[makeImageStreamKey(from)] = true
				}
			case buildapi.CustomBuildStrategyType:
				from := bc.Spec.Strategy.CustomStrategy.From
				if from.Kind == "ImageStreamTag" {
					imageStreams[makeImageStreamKey(from)] = true
				}
			}
		}
	}
	items := []runtime.Object{}
	// 2nd pass to remove ImageStreams not used by BuildConfigs
	for _, item := range result.List.Items {
		if is, ok := item.(*imageapi.ImageStream); ok {
			if _, ok := imageStreams[types.NamespacedName{is.Namespace, is.Name}.String()]; ok {
				items = append(items, is)
			}
		} else {
			items = append(items, item)
		}
	}
	result.List.Items = items
	return result
}

func makeImageStreamKey(ref kapi.ObjectReference) string {
	name, _, _ := imageapi.SplitImageStreamTag(ref.Name)
	return types.NamespacedName{ref.Namespace, name}.String()
}

// RunQuery executes the provided config and returns the result of the resolution.
func (c *AppConfig) RunQuery(out, errOut io.Writer) (*QueryResult, error) {
	c.ensureDockerSearcher()
	repositories, err := c.individualSourceRepositories()
	if err != nil {
		return nil, err
	}

	if c.AsList {
		if c.AsSearch {
			return nil, fmt.Errorf("--list and --search can't be used together")
		}
		if c.HasArguments() {
			return nil, fmt.Errorf("--list can't be used with arguments")
		}
		c.Components.Set("*")
	}

	components, repositories, environment, parameters, err := c.validate()
	if err != nil {
		return nil, err
	}

	if len(components) == 0 && !c.AsList {
		return nil, ErrNoInputs
	}

	errs := []error{}
	if len(repositories) > 0 {
		errs = append(errs, fmt.Errorf("--search can't be used with source code"))
	}
	if len(environment) > 0 {
		errs = append(errs, fmt.Errorf("--search can't be used with --env"))
	}
	if len(parameters) > 0 {
		errs = append(errs, fmt.Errorf("--search can't be used with --param"))
	}
	if len(errs) > 0 {
		return nil, errors.NewAggregate(errs)
	}

	if err := c.search(components); err != nil {
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

// run executes the provided config applying provided acceptors.
func (c *AppConfig) run(out, errOut io.Writer, acceptors app.Acceptors) (*AppResult, error) {
	c.ensureDockerSearcher()
	repositories, err := c.individualSourceRepositories()
	if err != nil {
		return nil, err
	}
	err = c.detectSource(repositories)
	if err != nil {
		return nil, err
	}
	components, repositories, environment, parameters, err := c.validate()
	if err != nil {
		return nil, err
	}
	if err := c.resolve(components); err != nil {
		return nil, err
	}

	// Couple source with resolved builder components if possible
	if err := c.ensureHasSource(components.NeedsSource(), repositories.NotUsed()); err != nil {
		return nil, err
	}
	// For source repos that are not yet coupled with a component, create components
	sourceComponents, err := c.componentsForRepos(repositories.NotUsed())
	if err != nil {
		return nil, err
	}
	// resolve the source repo components
	if err := c.resolve(sourceComponents); err != nil {
		return nil, err
	}
	components = append(components, sourceComponents...)

	glog.V(4).Infof("Code %v", repositories)
	glog.V(4).Infof("Components %v", components)

	if len(repositories) == 0 && len(components) == 0 {
		return nil, ErrNoInputs
	}

	if len(c.Name) > 0 {
		if err := c.validateEnforcedName(); err != nil {
			return nil, err
		}
	}

	if len(components.ImageComponentRefs()) > 1 && len(c.Name) > 0 {
		return nil, fmt.Errorf("only one component or source repository can be used when specifying a name")
	}

	pipelines, err := c.buildPipelines(components.ImageComponentRefs(), app.Environment(environment))
	if err != nil {
		return nil, err
	}

	objects := app.Objects{}
	accept := app.NewAcceptFirst()
	warned := make(map[string]struct{})
	for _, p := range pipelines {
		accepted, err := p.Objects(accept, acceptors)
		if err != nil {
			return nil, fmt.Errorf("can't setup %q: %v", p.From, err)
		}
		if p.Image != nil && p.Image.HasEmptyDir {
			if _, ok := warned[p.Image.Name]; !ok {
				fmt.Fprintf(errOut, "NOTICE: Image %q uses an EmptyDir volume. Data in EmptyDir volumes is not persisted across deployments.\n", p.Image.Name)
				warned[p.Image.Name] = struct{}{}
			}
		}
		objects = append(objects, accepted...)
	}

	objects = app.AddServices(objects, false)

	templateObjects, err := c.buildTemplates(components.TemplateComponentRefs(), app.Environment(parameters))
	if err != nil {
		return nil, err
	}
	objects = append(objects, templateObjects...)

	buildNames := []string{}
	for _, obj := range objects {
		switch t := obj.(type) {
		case *buildapi.BuildConfig:
			buildNames = append(buildNames, t.Name)
		}
	}

	name := c.Name
	if len(name) == 0 {
		for _, pipeline := range pipelines {
			if pipeline.Deployment != nil {
				name = pipeline.Deployment.Name
				break
			}
		}
	}

	return &AppResult{
		List:       &kapi.List{Items: objects},
		Name:       name,
		BuildNames: buildNames,
		HasSource:  len(repositories) != 0,
		Namespace:  c.originNamespace,
	}, nil
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

func (c *AppConfig) GetBuildEnvironment(environment app.Environment) app.Environment {
	if c.AddEnvironmentToBuild {
		return environment
	}
	return app.Environment{}
}
