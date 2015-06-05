package cmd

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"
	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"

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

// AppConfig contains all the necessary configuration for an application
type AppConfig struct {
	SourceRepositories util.StringList
	ContextDir         string

	Components         util.StringList
	ImageStreams       util.StringList
	DockerImages       util.StringList
	Templates          util.StringList
	TemplateFiles      util.StringList
	TemplateParameters util.StringList
	Groups             util.StringList
	Environment        util.StringList

	Name             string
	Strategy         string
	InsecureRegistry bool
	OutputDocker     bool

	dockerResolver       app.Resolver
	imageStreamResolver  app.Resolver
	templateResolver     app.Resolver
	templateFileResolver app.Resolver

	searcher app.Searcher
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
	dockerResolver := app.DockerRegistryResolver{
		Client: dockerregistry.NewClient(),
	}
	return &AppConfig{
		detector: app.SourceRepositoryEnumerator{
			Detectors: source.DefaultDetectors,
			Tester:    dockerfile.NewTester(),
		},
		dockerResolver: dockerResolver,
		searcher:       &simpleSearcher{dockerResolver},
		typer:          typer,
		mapper:         mapper,
		clientMapper:   clientMapper,
	}
}

func (c *AppConfig) dockerRegistryResolver() app.Resolver {
	return app.DockerRegistryResolver{
		Client:        dockerregistry.NewClient(),
		AllowInsecure: c.InsecureRegistry,
	}
}

func (c *AppConfig) ensureDockerResolver() {
	if c.dockerResolver == nil {
		c.dockerResolver = c.dockerRegistryResolver()
	}
}

// SetDockerClient sets the passed Docker client in the application configuration
func (c *AppConfig) SetDockerClient(dockerclient *docker.Client) {
	c.dockerResolver = app.DockerClientResolver{
		Client:           dockerclient,
		RegistryResolver: c.dockerRegistryResolver(),
		Insecure:         c.InsecureRegistry,
	}
}

// SetOpenShiftClient sets the passed OpenShift client in the application configuration
func (c *AppConfig) SetOpenShiftClient(osclient client.Interface, originNamespace string) {
	c.osclient = osclient
	c.originNamespace = originNamespace
	c.imageStreamResolver = app.ImageStreamResolver{
		Client:            osclient,
		ImageStreamImages: osclient,
		Namespaces:        []string{originNamespace, "openshift"},
	}
	c.templateResolver = app.TemplateResolver{
		Client: osclient,
		TemplateConfigsNamespacer: osclient,
		Namespaces:                []string{originNamespace, "openshift"},
	}
	c.templateFileResolver = &app.TemplateFileResolver{
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

// validate converts all of the arguments on the config into references to objects, or returns an error
func (c *AppConfig) validate() (app.ComponentReferences, []*app.SourceRepository, cmdutil.Environment, cmdutil.Environment, error) {
	b := &app.ReferenceBuilder{}
	for _, s := range c.SourceRepositories {
		b.AddSourceRepository(s)
	}
	b.AddComponents(c.DockerImages, func(input *app.ComponentInput) app.ComponentReference {
		input.Argument = fmt.Sprintf("--docker-image=%q", input.From)
		input.Resolver = c.dockerResolver
		return input
	})
	b.AddComponents(c.ImageStreams, func(input *app.ComponentInput) app.ComponentReference {
		input.Argument = fmt.Sprintf("--image=%q", input.From)
		input.Resolver = c.imageStreamResolver
		return input
	})
	b.AddComponents(c.Templates, func(input *app.ComponentInput) app.ComponentReference {
		input.Argument = fmt.Sprintf("--template=%q", input.From)
		input.Resolver = c.templateResolver
		return input
	})
	b.AddComponents(c.TemplateFiles, func(input *app.ComponentInput) app.ComponentReference {
		input.Argument = fmt.Sprintf("--file=%q", input.From)
		input.Resolver = c.templateFileResolver
		return input
	})
	b.AddComponents(c.Components, func(input *app.ComponentInput) app.ComponentReference {
		input.Resolver = app.PerfectMatchWeightedResolver{
			app.WeightedResolver{Resolver: c.imageStreamResolver, Weight: 0.0},
			app.WeightedResolver{Resolver: c.templateResolver, Weight: 0.0},
			app.WeightedResolver{Resolver: c.templateFileResolver, Weight: 0.0},
			app.WeightedResolver{Resolver: c.dockerResolver, Weight: 2.0},
		}
		return input
	})
	b.AddGroups(c.Groups)
	refs, repos, errs := b.Result()

	if len(repos) > 0 {
		repos[0].SetContextDir(c.ContextDir)
	}

	if len(c.Strategy) != 0 && len(repos) == 0 {
		errs = append(errs, fmt.Errorf("when --build is specified you must provide at least one source code location"))
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

// resolve the references to ensure they are all valid, and identify any images that don't match user input.
func (c *AppConfig) resolve(components app.ComponentReferences) error {
	errs := []error{}
	for _, ref := range components {
		if err := ref.Resolve(); err != nil {
			errs = append(errs, err)
			continue
		}
		switch input := ref.Input(); {
		case !input.ExpectToBuild && input.Match.Builder:
			if c.Strategy != "docker" {
				glog.Infof("Image %q is a builder, so a repository will be expected unless you also specify --build=docker", input)
				input.ExpectToBuild = true
			}
		case input.ExpectToBuild && input.Match.IsTemplate():
			// TODO: harder - break the template pieces and check if source code can be attached (look for a build config, build image, etc)
			errs = append(errs, fmt.Errorf("template with source code explicitly attached is not supported - you must either specify the template and source code separately or attach an image to the source code using the '[image]~[code]' form"))
			continue
		case input.ExpectToBuild && !input.Match.Builder:
			if len(c.Strategy) == 0 {
				errs = append(errs, fmt.Errorf("none of the images that match %q can build source code - check whether this is the image you want to use, then use --build=source to build using source or --build=docker to treat this as a Docker base image and set up a layered Docker build", ref))
				continue
			}
		}
	}
	return errors.NewAggregate(errs)
}

// ensureHasSource ensure every builder component has source code associated with it
func (c *AppConfig) ensureHasSource(components app.ComponentReferences, repositories []*app.SourceRepository) error {
	requiresSource := components.NeedsSource()
	notUsed := []string{}
	if len(requiresSource) > 0 {
		for _, repo := range repositories {
			if !repo.InUse() {
				notUsed = append(notUsed, repo.String())
			}
		}

		switch {
		case len(repositories) > 1:
			if len(requiresSource) == 1 {
				component := requiresSource[0]
				suggestions := ""

				for _, repo := range repositories {
					suggestions += fmt.Sprintf("%s~%s\n", component, repo)
				}
				return fmt.Errorf("there are multiple code locations provided - use one of the following suggestions to declare which code goes with the image:\n%s", suggestions)
			}
			reposNotUsed := strings.Join(notUsed, ",")
			return fmt.Errorf("the following images require source code: %s\n"+
				" and the following repositories are not used: %s\nUse '[image]~[repo]' to declare which code goes with which image", requiresSource, reposNotUsed)
		case len(repositories) == 1:
			glog.Infof("Using %q as the source for build", repositories[0])
			for _, component := range requiresSource {
				component.Input().Use(repositories[0])
				repositories[0].UsedBy(component)
			}
		default:
			if len(requiresSource) == 1 {
				return fmt.Errorf("the image %q will build source code, so you must specify a repository via --code", requiresSource[0])
			}
			return fmt.Errorf("you must provide at least one source code repository with --code for the images: %s", requiresSource)
		}
	}
	return nil
}

// detectSource tries to match each source repository to an image type
func (c *AppConfig) detectSource(repositories []*app.SourceRepository) (app.ComponentReferences, error) {
	errs := []error{}
	refs := app.ComponentReferences{}
	for _, repo := range repositories {
		// if the repository is being used by one of the images, we don't need to detect its type (unless we want to double check)
		if repo.InUse() {
			continue
		}
		path, err := repo.LocalPath()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		info, err := c.detector.Detect(path)
		if err != nil {
			if err == app.ErrNoLanguageDetected {
				err = fmt.Errorf("no language was detected for repository at %q; please specify a builder image to use with your repository: [builder-image]~%s", repo, repo)
			}
			errs = append(errs, err)
			continue
		}
		if info.Dockerfile != nil && (len(c.Strategy) == 0 || c.Strategy == "docker") {
			// TODO: this should be using the reference builder flow, possibly by moving detectSource up before other steps
			/*if from, ok := info.Dockerfile.GetDirective("FROM"); ok {
				input, _, err := NewComponentInput(from[0])
				if err != nil {
					errs = append(errs, err)
					continue
				}
				input.
			}*/
			ports, _ := info.Dockerfile.GetDirective("EXPOSE")
			exposedPorts := map[string]struct{}{}

			for _, p := range ports {
				exposedPorts[p] = struct{}{}
			}

			dockerImage := &imageapi.DockerImage{
				Config: imageapi.DockerConfig{
					ExposedPorts: exposedPorts,
				},
			}
			from, _ := info.Dockerfile.GetDirective("FROM")
			componentRef := &app.ComponentInput{
				Match: &app.ComponentMatch{
					Value: from[0],
					Image: dockerImage,
				},
				ExpectToBuild: true,
				Uses:          repo,
			}
			refs = append(refs, componentRef)
			repo.UsedBy(componentRef)
			repo.BuildWithDocker()
			continue
		}

		terms := info.Terms()
		matches, err := c.searcher.Search(terms)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if len(matches) == 0 {
			errs = append(errs, fmt.Errorf("we could not find any images that match the source repo %q (looked for: %v) and this repository does not have a Dockerfile - you'll need to choose a source builder image to continue", repo, terms))
			continue
		}
		componentRef := &app.ComponentInput{
			Match:         matches[0],
			ExpectToBuild: true,
			Uses:          repo,
		}
		repo.UsedBy(componentRef)
		refs = append(refs, componentRef)

	}
	return refs, errors.NewAggregate(errs)
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
			if !ref.Input().Match.IsImage() {
				continue
			}
			var pipeline *app.Pipeline
			if ref.Input().ExpectToBuild {
				glog.V(2).Infof("will use %q as the base image for a source build of %q", ref, ref.Input().Uses)
				input, err := app.InputImageFromMatch(ref.Input().Match)
				if err != nil {
					return nil, fmt.Errorf("can't build %q: %v", ref.Input(), err)
				}
				strategy, source, err := app.StrategyAndSourceForRepository(ref.Input().Uses, input)
				if err != nil {
					return nil, fmt.Errorf("can't build %q: %v", ref.Input(), err)
				}
				// Override resource names from the cli
				if len(c.Name) > 0 {
					source.Name = c.Name
				}
				if name, ok := (app.NameSuggestions{source, input}).SuggestName(); ok {
					source.Name, err = ensureValidUniqueName(names, name)
					if err != nil {
						return nil, err
					}
				}
				if pipeline, err = app.NewBuildPipeline(ref.Input().String(), input, c.OutputDocker, strategy, source); err != nil {
					return nil, fmt.Errorf("can't build %q: %v", ref.Input(), err)
				}
			} else {
				glog.V(2).Infof("will include %q", ref)
				input, err := app.InputImageFromMatch(ref.Input().Match)
				if name, ok := input.SuggestName(); ok {
					input.ObjectName, err = ensureValidUniqueName(names, name)
					if err != nil {
						return nil, err
					}
				}
				if err != nil {
					return nil, fmt.Errorf("can't include %q: %v", ref.Input(), err)
				}
				if pipeline, err = app.NewImagePipeline(ref.Input().String(), input); err != nil {
					return nil, fmt.Errorf("can't include %q: %v", ref.Input(), err)
				}
			}
			if err := pipeline.NeedsDeployment(environment, c.Name); err != nil {
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
		if !ref.Input().Match.IsTemplate() {
			continue
		}

		tpl := ref.Input().Match.Template

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

	BuildNames []string
	HasSource  bool
	Namespace  string
}

// RunAll executes the provided config to generate all objects.
func (c *AppConfig) RunAll(out io.Writer) (*AppResult, error) {
	c.ensureDockerResolver()
	components, repositories, environment, parameters, err := c.validate()
	if err != nil {
		return nil, err
	}
	if len(repositories) == 0 && len(components) == 0 {
		return nil, ErrNoInputs
	}

	return c.run(out, app.Acceptors{app.NewAcceptUnique(c.typer), app.AcceptNew},
		components, repositories, environment, parameters)
}

// RunBuilds executes the provided config to generate just builds.
func (c *AppConfig) RunBuilds(out io.Writer) (*AppResult, error) {
	components, repositories, environment, parameters, err := c.validate()
	if err != nil {
		return nil, err
	}
	if len(repositories) == 0 {
		return nil, ErrNoInputs
	}

	return c.run(out, app.Acceptors{app.NewAcceptBuildConfigs(c.typer), app.NewAcceptUnique(c.typer), app.AcceptNew},
		components, repositories, environment, parameters)
}

// run executes the provided config applying provided acceptors.
func (c *AppConfig) run(out io.Writer, acceptors app.Acceptors, components app.ComponentReferences,
	repositories []*app.SourceRepository, environment, parameters cmdutil.Environment) (*AppResult, error) {
	if err := c.resolve(components); err != nil {
		return nil, err
	}

	if err := c.ensureHasSource(components, repositories); err != nil {
		return nil, err
	}

	glog.V(4).Infof("Code %v", repositories)
	glog.V(4).Infof("Components %v", components)

	// TODO: Source detection needs to happen before components
	//       are validated and resolved.
	srcComponents, err := c.detectSource(repositories)
	if err != nil {
		return nil, err
	}

	components = append(components, srcComponents...)

	pipelines, err := c.buildPipelines(components, app.Environment(environment))
	if err != nil {
		return nil, err
	}

	objects := app.Objects{}
	accept := app.NewAcceptFirst()
	for _, p := range pipelines {
		accepted, err := p.Objects(accept, acceptors)
		if err != nil {
			return nil, fmt.Errorf("can't setup %q: %v", p.From, err)
		}
		objects = append(objects, accepted...)
	}

	objects = app.AddServices(objects)

	templateObjects, err := c.buildTemplates(components, app.Environment(parameters))
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

	return &AppResult{
		List:       &kapi.List{Items: objects},
		BuildNames: buildNames,
		HasSource:  len(repositories) != 0,
		Namespace:  c.originNamespace,
	}, nil
}

// simpleSearcher resolves known builder images for source code
// TODO: eventually needs to be replaced by a more sophisticated searcher
type simpleSearcher struct {
	resolver app.Resolver
}

// Search takes the first term if it exists and tries to match it to one
// of the known builder images
func (s *simpleSearcher) Search(terms []string) ([]*app.ComponentMatch, error) {
	if len(terms) == 0 {
		return nil, fmt.Errorf("No search terms were specified.")
	}
	term := terms[0]
	builder := app.BuilderForPlatform(term)
	if len(builder) == 0 {
		return nil, fmt.Errorf("No matching image found for %s", term)
	}
	match, err := s.resolver.Resolve(builder)
	return []*app.ComponentMatch{match}, err
}

type mockSearcher struct{}

// Search takes the first term if it exists and tries to match it to one
// of the known builder images. This is a mock function.
func (mockSearcher) Search(terms []string) ([]*app.ComponentMatch, error) {
	for _, term := range terms {
		term = strings.ToLower(term)
		switch term {
		case "redhat/mysql:5.6":
			return []*app.ComponentMatch{
				{
					Value:       term,
					Argument:    "redhat/mysql:5.6",
					Name:        "MySQL 5.6",
					Description: "The Open Source SQL database",
				},
			}, nil
		case "mysql", "mysql5", "mysql-5", "mysql-5.x":
			return []*app.ComponentMatch{
				{
					Value:       term,
					Argument:    "redhat/mysql:5.6",
					Name:        "MySQL 5.6",
					Description: "The Open Source SQL database",
				},
				{
					Value:       term,
					Argument:    "mysql",
					Name:        "MySQL 5.X",
					Description: "Something out there on the Docker Hub.",
				},
			}, nil
		case "php", "php-5", "php5", "redhat/php:5", "redhat/php-5":
			return []*app.ComponentMatch{
				{
					Value:       term,
					Argument:    "redhat/php:5",
					Name:        "PHP 5.5",
					Description: "A fast and easy to use scripting language for building websites.",
					Builder:     true,
				},
			}, nil
		case "ruby":
			return []*app.ComponentMatch{
				{
					Value:       term,
					Argument:    "redhat/ruby:2",
					Name:        "Ruby 2.0",
					Description: "A fast and easy to use scripting language for building websites.",
					Builder:     true,
				},
			}, nil
		}
	}
	return []*app.ComponentMatch{}, nil
}
