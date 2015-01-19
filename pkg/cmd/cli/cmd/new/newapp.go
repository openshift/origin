package new

import (
	"fmt"
	"io"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	dockerutil "github.com/openshift/origin/pkg/cmd/util/docker"
	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/generate/dockerfile"
	"github.com/openshift/origin/pkg/generate/source"
)

type newAppConfig struct {
	SourceRepositories util.StringList

	Components   util.StringList
	ImageStreams util.StringList
	DockerImages util.StringList
	Groups       util.StringList
	Environment  util.StringList

	TypeOfBuild string

	localDockerResolver Resolver
	imageStreamResolver Resolver

	searcher Searcher
	detector Detector
}

const longNewAppDescription = `
Create a new application in OpenShift by specifying source code, templates, and/or images.

Examples:
  $ osc new-app .
  <try to create an application based on the source code in the current directory>

  $ osc new-app mysql
  <use the public DockerHub MySQL image to create an app>

  $ osc new-app myregistry.com/mycompany/mysql
  <use a MySQL image in a private registry to create an app>

  $ osc new-app openshift/ruby-20-centos~git@github.com/mfojtik/sinatra-app-example
  <build an application using the OpenShift Ruby DockerHub image and an example repo>`

func NewCmdNewApplication(f *cmd.Factory, out io.Writer) *cobra.Command {
	config := newAppConfig{
		searcher: &mockSearcher{},
		detector: sourceRepositoryEnumerator{
			detectors: source.DefaultDetectors,
			tester:    dockerfile.NewTester(),
		},
	}
	helper := dockerutil.NewHelper()

	cmd := &cobra.Command{
		Use:   "new-app <components> [--code=<path|url>]",
		Short: "Create a new application",
		Long:  longNewAppDescription,

		Run: func(c *cobra.Command, args []string) {
			if dockerclient, _, err := helper.GetClient(); err == nil {
				config.localDockerResolver = dockerClientResolver{dockerclient}
				config.localDockerResolver = dockerRegistryResolver{dockerregistry.NewClient()}
			}
			if osclient, _, err := f.Clients(c); err == nil {
				config.imageStreamResolver = imageStreamResolver{
					client:     osclient,
					images:     osclient,
					namespaces: []string{cmd.GetOriginNamespace(c), "default"},
				}
			} else {
				glog.Warningf("error getting client: %v", err)
			}
			unknown := config.addArguments(args)
			if len(unknown) != 0 {
				glog.Fatalf("Did not recognize the following arguments: %v", unknown)
			}
			if err := config.Run(f, out, c.Help); err != nil {
				if errs, ok := err.(errlist); ok {
					if len(errs.Errors()) == 1 {
						err = errs.Errors()[0]
					}
				}
				if usage, ok := err.(UsageError); ok {
					glog.Fatal(usage.UsageError(c.CommandPath()))
				}
				glog.Fatalf("Error: %v", err)
			}
		},
	}

	cmd.Flags().Var(&config.SourceRepositories, "code", "Source code to use to build this application.")
	cmd.Flags().VarP(&config.ImageStreams, "image", "i", "Name of an OpenShift image repository to use in the app.")
	cmd.Flags().Var(&config.DockerImages, "docker-image", "Name of a Docker image to include in the app.")
	cmd.Flags().Var(&config.Groups, "group", "Indicate components that should be grouped together as <comp1>+<comp2>.")
	cmd.Flags().VarP(&config.Environment, "env", "e", "Specify key value pairs of environment variables to set into each container.")
	cmd.Flags().StringVar(&config.TypeOfBuild, "build", "", "Specify the type of build to use if you don't want to detect (docker|source)")
	return cmd
}

type UsageError interface {
	UsageError(commandName string) string
}

// TODO: replace with upstream converting [1]error to error
type errlist interface {
	Errors() []error
}

// addArguments converts command line arguments into the appropriate bucket based on what they look like
func (c *newAppConfig) addArguments(args []string) []string {
	unknown := []string{}
	for _, s := range args {
		switch {
		case cmdutil.IsEnvironmentArgument(s):
			c.Environment = append(c.Environment, s)
		case isPossibleSourceRepository(s):
			c.SourceRepositories = append(c.SourceRepositories, s)
		case isComponentReference(s):
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
func (c *newAppConfig) validate() (ComponentReferences, []*SourceRepository, cmdutil.Environment, error) {
	b := &ReferenceBuilder{}
	for _, s := range c.SourceRepositories {
		b.AddSourceRepository(s)
	}
	b.AddImages(c.DockerImages, func(input *ComponentInput) ComponentReference {
		input.Argument = fmt.Sprintf("--docker-image=%q", input.From)
		input.Resolver = c.localDockerResolver
		return input
	})
	b.AddImages(c.ImageStreams, func(input *ComponentInput) ComponentReference {
		input.Argument = fmt.Sprintf("--image=%q", input.From)
		input.Resolver = c.imageStreamResolver
		return input
	})
	b.AddImages(c.Components, func(input *ComponentInput) ComponentReference {
		input.Resolver = PerfectMatchWeightedResolver{
			WeightedResolver{Resolver: c.imageStreamResolver, Weight: 0.0},
			WeightedResolver{Resolver: c.localDockerResolver, Weight: 0.0},
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

	return refs, repos, env, util.SliceToError(errs)
}

// resolve the references to ensure they are all valid, and identify any images that don't match user input.
func (c *newAppConfig) resolve(components ComponentReferences) error {
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
	return util.SliceToError(errs)
}

// ensureHasSource ensure every builder component has source code associated with it
func (c *newAppConfig) ensureHasSource(components ComponentReferences, repositories []*SourceRepository) error {
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
func (c *newAppConfig) detectSource(repositories []*SourceRepository) error {
	errs := []error{}
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
		errs = append(errs, fmt.Errorf("found the following possible images to use to build this source repository: %v - to continue, you'll need to specify which image to use with %q", matches, repo))
	}
	return util.SliceToError(errs)
}

// buildPipelines converts a set of resolved, valid references into pipelines.
func (c *newAppConfig) buildPipelines(components ComponentReferences, environment app.Environment) (app.PipelineGroup, error) {
	pipelines := app.PipelineGroup{}
	for _, group := range components.Group() {
		glog.V(2).Infof("found group: %#v", group)
		common := app.PipelineGroup{}
		for _, ref := range group {

			var pipeline *app.Pipeline
			if ref.Input().ExpectToBuild {
				glog.V(2).Infof("will use %q as the base image for a source build of %q", ref, ref.Input().Uses)
				input, err := InputImageFromMatch(ref.Input().Match)
				if err != nil {
					return nil, fmt.Errorf("can't build %q: %v", ref.Input(), err)
				}
				strategy, source, err := StrategyAndSourceForRepository(ref.Input().Uses)
				if err != nil {
					return nil, fmt.Errorf("can't build %q: %v", ref.Input(), err)
				}
				if pipeline, err = app.NewBuildPipeline(ref.Input().String(), input, strategy, source); err != nil {
					return nil, fmt.Errorf("can't build %q: %v", ref.Input(), err)
				}

			} else {
				glog.V(2).Infof("will include %q", ref)
				input, err := InputImageFromMatch(ref.Input().Match)
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

// Run executes the provided config.
func (c *newAppConfig) Run(f *cmd.Factory, out io.Writer, helpFn func() error) error {
	components, repositories, environment, err := c.validate()
	if err != nil {
		return err
	}

	hasSource := len(repositories) != 0
	hasImages := len(components) != 0
	if !hasSource && !hasImages {
		// display help page
		// TODO: return usage error, which should trigger help display
		return helpFn()
	}

	if err := c.resolve(components); err != nil {
		return err
	}

	if err := c.ensureHasSource(components, repositories); err != nil {
		return err
	}

	glog.V(4).Infof("Code %v", repositories)
	glog.V(4).Infof("Images %v", components)

	if err := c.detectSource(repositories); err != nil {
		return err
	}

	pipelines, err := c.buildPipelines(components, app.Environment(environment))
	if err != nil {
		return err
	}

	objects := app.Objects{}
	accept := app.NewAcceptFirst()
	for _, p := range pipelines {
		obj, err := p.Objects(accept)
		if err != nil {
			return fmt.Errorf("can't setup %q: %v", p.From, err)
		}
		objects = append(objects, obj...)
	}

	objects = app.AddServices(objects)

	list := &kapi.List{Items: objects}
	p, err := kubectl.GetPrinter("yaml", "", "v1beta1", kapi.Scheme, nil)
	if err != nil {
		return err
	}
	return p.PrintObj(list, out)
}
