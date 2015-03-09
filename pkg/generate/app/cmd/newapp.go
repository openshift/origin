package cmd

import (
	"fmt"
	"io"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
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
)

type AppConfig struct {
	SourceRepositories util.StringList

	Components   util.StringList
	ImageStreams util.StringList
	DockerImages util.StringList
	Groups       util.StringList
	Environment  util.StringList

	TypeOfBuild string

	dockerResolver      app.Resolver
	imageStreamResolver app.Resolver

	searcher app.Searcher
	detector app.Detector
}

type UsageError interface {
	UsageError(commandName string) string
}

// TODO: replace with upstream converting [1]error to error
type errlist interface {
	Errors() []error
}

func NewAppConfig() *AppConfig {
	dockerResolver := app.DockerRegistryResolver{dockerregistry.NewClient()}
	return &AppConfig{
		detector: app.SourceRepositoryEnumerator{
			Detectors: source.DefaultDetectors,
			Tester:    dockerfile.NewTester(),
		},
		dockerResolver: dockerResolver,
		searcher:       &simpleSearcher{dockerResolver},
	}
}

func (c *AppConfig) SetDockerClient(dockerclient *docker.Client) {
	c.dockerResolver = app.DockerClientResolver{
		Client: dockerclient,

		RegistryResolver: c.dockerResolver,
	}
}

func (c *AppConfig) SetOpenShiftClient(osclient client.Interface, originNamespace string) {
	c.imageStreamResolver = app.ImageStreamResolver{
		Client:     osclient,
		Images:     osclient,
		Namespaces: []string{originNamespace, "default"},
	}
}

// addArguments converts command line arguments into the appropriate bucket based on what they look like
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
func (c *AppConfig) validate() (app.ComponentReferences, []*app.SourceRepository, cmdutil.Environment, error) {
	b := &app.ReferenceBuilder{}
	for _, s := range c.SourceRepositories {
		b.AddSourceRepository(s)
	}
	b.AddImages(c.DockerImages, func(input *app.ComponentInput) app.ComponentReference {
		input.Argument = fmt.Sprintf("--docker-image=%q", input.From)
		input.Resolver = c.dockerResolver
		return input
	})
	b.AddImages(c.ImageStreams, func(input *app.ComponentInput) app.ComponentReference {
		input.Argument = fmt.Sprintf("--image=%q", input.From)
		input.Resolver = c.imageStreamResolver
		return input
	})
	b.AddImages(c.Components, func(input *app.ComponentInput) app.ComponentReference {
		input.Resolver = app.PerfectMatchWeightedResolver{
			app.WeightedResolver{Resolver: c.imageStreamResolver, Weight: 0.0},
			app.WeightedResolver{Resolver: c.dockerResolver, Weight: 0.0},
		}
		return input
	})
	b.AddGroups(c.Groups)
	refs, repos, errs := b.Result()
	if len(c.TypeOfBuild) != 0 && len(repos) == 0 {
		errs = append(errs, fmt.Errorf("when --build is specified you must provide at least one source code location"))
	}

	env, duplicate, envErrs := cmdutil.ParseEnvironmentArguments(c.Environment)
	for _, s := range duplicate {
		glog.V(1).Infof("The environment variable %q was overwritten", s)
	}
	errs = append(errs, envErrs...)

	return refs, repos, env, errors.NewAggregate(errs)
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
			if c.TypeOfBuild != "docker" {
				glog.Infof("Image %q is a builder, so a repository will be expected unless you also specify --build=docker", input)
				input.ExpectToBuild = true
			}
		case input.ExpectToBuild && !input.Match.Builder:
			if len(c.TypeOfBuild) == 0 {
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
	if len(requiresSource) > 0 {
		switch {
		case len(repositories) > 1:
			// TODO: harder problem - need to match repos up
			if len(requiresSource) == 1 {
				// TODO: print all suggestions
				return fmt.Errorf("there are multiple code locations provided - use '%s~<repo>' to declare which code goes with the image", requiresSource[0])
			}
			// TODO: indicate which args don't match, and which repos don't match
			return fmt.Errorf("there are multiple code locations provided - use '[image]~[repo]' to declare which code goes with which image")
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
			// TODO: array of pointers won't print correctly
			return fmt.Errorf("you must provide at least one source code repository with --code for the images: %v", requiresSource)
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
			errs = append(errs, err)
			continue
		}
		if info.Dockerfile != nil {
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

// buildPipelines converts a set of resolved, valid references into pipelines.
func (c *AppConfig) buildPipelines(components app.ComponentReferences, environment app.Environment) (app.PipelineGroup, error) {
	pipelines := app.PipelineGroup{}
	for _, group := range components.Group() {
		glog.V(2).Infof("found group: %#v", group)
		common := app.PipelineGroup{}
		for _, ref := range group {

			var pipeline *app.Pipeline
			if ref.Input().ExpectToBuild {
				glog.V(2).Infof("will use %q as the base image for a source build of %q", ref, ref.Input().Uses)
				input, err := app.InputImageFromMatch(ref.Input().Match)
				if err != nil {
					return nil, fmt.Errorf("can't build %q: %v", ref.Input(), err)
				}
				strategy, source, err := app.StrategyAndSourceForRepository(ref.Input().Uses)
				if err != nil {
					return nil, fmt.Errorf("can't build %q: %v", ref.Input(), err)
				}
				if pipeline, err = app.NewBuildPipeline(ref.Input().String(), input, strategy, source); err != nil {
					return nil, fmt.Errorf("can't build %q: %v", ref.Input(), err)
				}

			} else {
				glog.V(2).Infof("will include %q", ref)
				input, err := app.InputImageFromMatch(ref.Input().Match)
				if err != nil {
					return nil, fmt.Errorf("can't include %q: %v", ref.Input(), err)
				}
				if pipeline, err = app.NewImagePipeline(ref.Input().String(), input); err != nil {
					return nil, fmt.Errorf("can't include %q: %v", ref.Input(), err)
				}
			}

			if err := pipeline.NeedsDeployment(environment); err != nil {
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

var ErrNoInputs = fmt.Errorf("no inputs provided")

type AppResult struct {
	List *kapi.List

	BuildNames []string
	HasSource  bool
}

// Run executes the provided config.
func (c *AppConfig) Run(out io.Writer) (*AppResult, error) {
	components, repositories, environment, err := c.validate()
	if err != nil {
		return nil, err
	}

	hasSource := len(repositories) != 0
	hasImages := len(components) != 0
	if !hasSource && !hasImages {
		return nil, ErrNoInputs
	}

	if err := c.resolve(components); err != nil {
		return nil, err
	}

	if err := c.ensureHasSource(components, repositories); err != nil {
		return nil, err
	}

	glog.V(4).Infof("Code %v", repositories)
	glog.V(4).Infof("Images %v", components)

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
		obj, err := p.Objects(accept)
		if err != nil {
			return nil, fmt.Errorf("can't setup %q: %v", p.From, err)
		}
		objects = append(objects, obj...)
	}

	objects = app.AddServices(objects)

	buildNames := []string{}
	for _, obj := range objects {
		switch t := obj.(type) {
		case *buildapi.BuildConfig:
			buildNames = append(buildNames, t.Name)
		}
	}

	list := &kapi.List{Items: objects}
	return &AppResult{
		List:       list,
		BuildNames: buildNames,
		HasSource:  hasSource,
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
	term := strings.ToLower(terms[0])
	builder := ""
	switch term {
	case "jee":
		builder = "openshift/wildfly-8-centos"
	case "ruby":
		builder = "openshift/ruby-20-centos7"
	case "nodejs":
		builder = "openshift/node-0-10-centos"
	default:
		return nil, fmt.Errorf("No matching image found for %s", term)
	}
	match, err := s.resolver.Resolve(builder)
	return []*app.ComponentMatch{match}, err
}

type mockSearcher struct{}

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
