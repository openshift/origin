package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang/glog"
	kutilerrors "k8s.io/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/generate/app"
	dockerfileutil "github.com/openshift/origin/pkg/util/docker/dockerfile"
)

// Resolvers are used to identify source repositories, images, or templates in different contexts
type Resolvers struct {
	DockerSearcher                  app.Searcher
	ImageStreamSearcher             app.Searcher
	ImageStreamByAnnotationSearcher app.Searcher
	TemplateSearcher                app.Searcher
	TemplateFileSearcher            app.Searcher

	AllowMissingImages bool

	Detector app.Detector
}

func (r *Resolvers) ImageSourceResolver() app.Resolver {
	resolver := app.PerfectMatchWeightedResolver{}
	if r.ImageStreamByAnnotationSearcher != nil {
		resolver = append(resolver, app.WeightedResolver{Searcher: r.ImageStreamByAnnotationSearcher, Weight: 0.0})
	}
	if r.ImageStreamSearcher != nil {
		resolver = append(resolver, app.WeightedResolver{Searcher: r.ImageStreamSearcher, Weight: 1.0})
	}
	if r.DockerSearcher != nil {
		resolver = append(resolver, app.WeightedResolver{Searcher: r.DockerSearcher, Weight: 2.0})
	}
	return resolver
}

func (r *Resolvers) DockerfileResolver() app.Resolver {
	resolver := app.PerfectMatchWeightedResolver{}
	if r.ImageStreamSearcher != nil {
		resolver = append(resolver, app.WeightedResolver{Searcher: r.ImageStreamSearcher, Weight: 0.0})
	}
	if r.DockerSearcher != nil {
		resolver = append(resolver, app.WeightedResolver{Searcher: r.DockerSearcher, Weight: 1.0})
	}
	if r.AllowMissingImages {
		resolver = append(resolver, app.WeightedResolver{Searcher: &app.MissingImageSearcher{}, Weight: 100.0})
	}
	return resolver
}

// TODO: why does this differ from ImageSourceResolver?
func (r *Resolvers) SourceResolver() app.Resolver {
	resolver := app.PerfectMatchWeightedResolver{}

	if r.ImageStreamSearcher != nil {
		resolver = append(resolver, app.WeightedResolver{Searcher: r.ImageStreamSearcher, Weight: 0.0})
	}
	if r.ImageStreamByAnnotationSearcher != nil {
		resolver = append(resolver, app.WeightedResolver{Searcher: r.ImageStreamByAnnotationSearcher, Weight: 1.0})
	}
	if r.DockerSearcher != nil {
		resolver = append(resolver, app.WeightedResolver{Searcher: r.DockerSearcher, Weight: 2.0})
	}
	return resolver
}

// ComponentInputs are transformed into ResolvedComponents
type ComponentInputs struct {
	SourceRepositories []string

	Components    []string
	ImageStreams  []string
	DockerImages  []string
	Templates     []string
	TemplateFiles []string

	Groups []string
}

// ResolvedComponents is the input to generation
type ResolvedComponents struct {
	Components   app.ComponentReferences
	Repositories app.SourceRepositories
}

// Resolve transforms unstructured inputs (component names, templates, images) into
// a set of resolved components, or returns an error.
func Resolve(r *Resolvers, c *ComponentInputs, g *GenerationInputs) (*ResolvedComponents, error) {
	b := &app.ReferenceBuilder{}

	if err := AddComponentInputsToRefBuilder(b, r, c, g); err != nil {
		return nil, err
	}
	components, repositories, errs := b.Result()
	if len(errs) > 0 {
		return nil, kutilerrors.NewAggregate(errs)
	}

	// TODO: the second half of this method is potentially splittable - each chunk below amends or qualifies
	// the inputs provided by the user (mostly via flags). c is cleared to prevent it from being used accidentally.
	c = nil

	// set context dir on all repositories
	for _, repo := range repositories {
		repo.SetContextDir(g.ContextDir)
	}

	if len(g.Strategy) != 0 && len(repositories) == 0 && !g.BinaryBuild {
		return nil, errors.New("when --strategy is specified you must provide at least one source code location")
	}

	if g.BinaryBuild && (len(repositories) > 0 || components.HasSource()) {
		return nil, errors.New("specifying binary builds and source repositories at the same time is not allowed")
	}

	// Add source components if source-image points to another location
	// TODO: image sources aren't really "source repositories" and we should probably find another way to
	// represent them
	var err error
	var imageComp app.ComponentReference
	imageComp, repositories, err = AddImageSourceRepository(repositories, r.ImageSourceResolver(), g)
	if err != nil {
		return nil, err
	}
	componentsIncludingImageComps := components
	if imageComp != nil {
		componentsIncludingImageComps = append(components, imageComp)
	}

	if err := componentsIncludingImageComps.Resolve(); err != nil {
		return nil, err
	}

	// If any references are potentially ambiguous in the future, force the user to provide the
	// unambiguous input.
	if err := detectPartialMatches(componentsIncludingImageComps); err != nil {
		return nil, err
	}

	// Guess at the build types
	components, err = InferBuildTypes(components, g)
	if err != nil {
		return nil, err
	}

	// Couple source with resolved builder components if possible
	if err := EnsureHasSource(components.NeedsSource(), repositories.NotUsed(), g); err != nil {
		return nil, err
	}

	// For source repos that are not yet linked to a component, create components
	sourceComponents, err := AddMissingComponentsToRefBuilder(b, repositories.NotUsed(), r.DockerfileResolver(), r.SourceResolver(), g)
	if err != nil {
		return nil, err
	}

	// Resolve any new source components added
	if err := sourceComponents.Resolve(); err != nil {
		return nil, err
	}
	components = append(components, sourceComponents...)

	glog.V(4).Infof("Code [%v]", repositories)
	glog.V(4).Infof("Components [%v]", components)

	return &ResolvedComponents{
		Components:   components,
		Repositories: repositories,
	}, nil
}

// AddSourceRepositoriesToRefBuilder adds the provided repositories to the reference builder, identifies which
// should be built using Docker, and then returns the full list of source repositories.
func AddSourceRepositoriesToRefBuilder(b *app.ReferenceBuilder, repos []string, g *GenerationInputs) (app.SourceRepositories, error) {
	for _, s := range repos {
		if repo, ok := b.AddSourceRepository(s); ok {
			repo.SetContextDir(g.ContextDir)
			if g.Strategy == "docker" {
				repo.BuildWithDocker()
			}
		}
	}
	if len(g.Dockerfile) > 0 {
		if len(g.Strategy) != 0 && g.Strategy != "docker" {
			return nil, errors.New("when directly referencing a Dockerfile, the strategy must must be 'docker'")
		}
		if err := AddDockerfileToSourceRepositories(b, g.Dockerfile); err != nil {
			return nil, err
		}
	}
	_, result, errs := b.Result()
	return result, kutilerrors.NewAggregate(errs)
}

// AddDockerfile adds a Dockerfile passed in the command line to the reference
// builder.
func AddDockerfileToSourceRepositories(b *app.ReferenceBuilder, dockerfile string) error {
	_, repos, errs := b.Result()
	if err := kutilerrors.NewAggregate(errs); err != nil {
		return err
	}
	switch len(repos) {
	case 0:
		// Create a new SourceRepository with the Dockerfile.
		repo, err := app.NewSourceRepositoryForDockerfile(dockerfile)
		if err != nil {
			return fmt.Errorf("provided Dockerfile is not valid: %v", err)
		}
		b.AddExistingSourceRepository(repo)
	case 1:
		// Add the Dockerfile to the existing SourceRepository, so that
		// eventually we generate a single BuildConfig with multiple
		// sources.
		if err := repos[0].AddDockerfile(dockerfile); err != nil {
			return fmt.Errorf("provided Dockerfile is not valid: %v", err)
		}
	default:
		// Invalid.
		return errors.New("--dockerfile cannot be used with multiple source repositories")
	}
	return nil
}

// DetectSource runs a code detector on the passed in repositories to obtain a SourceRepositoryInfo
func DetectSource(repositories []*app.SourceRepository, d app.Detector, g *GenerationInputs) error {
	errs := []error{}
	for _, repo := range repositories {
		err := repo.Detect(d, g.Strategy == "docker")
		if err != nil {
			if g.Strategy == "docker" && err == app.ErrNoLanguageDetected {
				errs = append(errs, ErrNoDockerfileDetected)
			} else {
				errs = append(errs, err)
			}
			continue
		}
	}
	return kutilerrors.NewAggregate(errs)
}

// AddComponentInputsToRefBuilder set up the components to be used by the reference builder.
func AddComponentInputsToRefBuilder(b *app.ReferenceBuilder, r *Resolvers, c *ComponentInputs, g *GenerationInputs) error {
	// lookup source repositories first (before processing the component inputs)
	repositories, err := AddSourceRepositoriesToRefBuilder(b, c.SourceRepositories, g)
	if err != nil {
		return err
	}

	// identify the types of the provided source locations
	if err := DetectSource(repositories, r.Detector, g); err != nil {
		return err
	}

	b.AddComponents(c.DockerImages, func(input *app.ComponentInput) app.ComponentReference {
		input.Argument = fmt.Sprintf("--docker-image=%q", input.From)
		input.Searcher = r.DockerSearcher
		if r.DockerSearcher != nil {
			resolver := app.PerfectMatchWeightedResolver{}
			resolver = append(resolver, app.WeightedResolver{Searcher: r.DockerSearcher, Weight: 0.0})
			if r.AllowMissingImages {
				resolver = append(resolver, app.WeightedResolver{Searcher: app.MissingImageSearcher{}, Weight: 100.0})
			}
			input.Resolver = resolver
		}
		return input
	})
	b.AddComponents(c.ImageStreams, func(input *app.ComponentInput) app.ComponentReference {
		input.Argument = fmt.Sprintf("--image-stream=%q", input.From)
		input.Searcher = r.ImageStreamSearcher
		if r.ImageStreamSearcher != nil {
			resolver := app.PerfectMatchWeightedResolver{
				app.WeightedResolver{Searcher: r.ImageStreamSearcher},
			}
			input.Resolver = resolver
		}
		return input
	})
	b.AddComponents(c.Templates, func(input *app.ComponentInput) app.ComponentReference {
		input.Argument = fmt.Sprintf("--template=%q", input.From)
		input.Searcher = r.TemplateSearcher
		if r.TemplateSearcher != nil {
			input.Resolver = app.HighestUniqueScoreResolver{Searcher: r.TemplateSearcher}
		}
		return input
	})
	b.AddComponents(c.TemplateFiles, func(input *app.ComponentInput) app.ComponentReference {
		input.Argument = fmt.Sprintf("--file=%q", input.From)
		if r.TemplateFileSearcher != nil {
			input.Resolver = app.FirstMatchResolver{Searcher: r.TemplateFileSearcher}
		}
		input.Searcher = r.TemplateFileSearcher
		return input
	})
	b.AddComponents(c.Components, func(input *app.ComponentInput) app.ComponentReference {
		resolver := app.PerfectMatchWeightedResolver{}
		searcher := app.MultiWeightedSearcher{}
		if r.ImageStreamSearcher != nil {
			resolver = append(resolver, app.WeightedResolver{Searcher: r.ImageStreamSearcher, Weight: 0.0})
			searcher = append(searcher, app.WeightedSearcher{Searcher: r.ImageStreamSearcher, Weight: 0.0})
		}
		if r.TemplateSearcher != nil && !input.ExpectToBuild {
			resolver = append(resolver, app.WeightedResolver{Searcher: r.TemplateSearcher, Weight: 0.0})
			searcher = append(searcher, app.WeightedSearcher{Searcher: r.TemplateSearcher, Weight: 0.0})
		}
		if r.TemplateFileSearcher != nil && !input.ExpectToBuild {
			resolver = append(resolver, app.WeightedResolver{Searcher: r.TemplateFileSearcher, Weight: 0.0})
		}
		if r.DockerSearcher != nil {
			resolver = append(resolver, app.WeightedResolver{Searcher: r.DockerSearcher, Weight: 2.0})
			searcher = append(searcher, app.WeightedSearcher{Searcher: r.DockerSearcher, Weight: 1.0})
		}
		if r.AllowMissingImages {
			resolver = append(resolver, app.WeightedResolver{Searcher: app.MissingImageSearcher{}, Weight: 100.0})
		}
		input.Resolver = resolver
		input.Searcher = searcher
		return input
	})
	b.AddGroups(c.Groups)

	return nil
}

func AddImageSourceRepository(sourceRepos app.SourceRepositories, r app.Resolver, g *GenerationInputs) (app.ComponentReference, app.SourceRepositories, error) {
	if len(g.SourceImage) == 0 {
		return nil, sourceRepos, nil
	}

	paths := strings.SplitN(g.SourceImagePath, ":", 2)
	var sourcePath, destPath string
	switch len(paths) {
	case 1:
		sourcePath = paths[0]
	case 2:
		sourcePath = paths[0]
		destPath = paths[1]
	}

	compRef, _, err := app.NewComponentInput(g.SourceImage)
	if err != nil {
		return nil, nil, err
	}
	compRef.Resolver = r

	switch len(sourceRepos) {
	case 0:
		sourceRepos = append(sourceRepos, app.NewImageSourceRepository(compRef, sourcePath, destPath))
	case 1:
		sourceRepos[0].SetSourceImage(compRef)
		sourceRepos[0].SetSourceImagePath(sourcePath, destPath)
	default:
		return nil, nil, errors.New("--image-source cannot be used with multiple source repositories")
	}

	return compRef, sourceRepos, nil
}

func detectPartialMatches(components app.ComponentReferences) error {
	errs := []error{}
	for _, ref := range components {
		input := ref.Input()
		if input.ResolvedMatch.Score != 0.0 {
			errs = append(errs, fmt.Errorf("component %q had only a partial match of %q - if this is the value you want to use, specify it explicitly", input.From, input.ResolvedMatch.Name))
		}
	}
	return kutilerrors.NewAggregate(errs)
}

// InferBuildTypes infers build status and mismatches between source and docker builders
func InferBuildTypes(components app.ComponentReferences, g *GenerationInputs) (app.ComponentReferences, error) {
	errs := []error{}
	for _, ref := range components {
		input := ref.Input()

		// identify whether the input is a builder and whether generation is requested
		input.ResolvedMatch.Builder = app.IsBuilderMatch(input.ResolvedMatch)
		generatorInput, err := app.GeneratorInputFromMatch(input.ResolvedMatch)
		if err != nil && !g.AllowGenerationErrors {
			errs = append(errs, err)
			continue
		}
		input.ResolvedMatch.GeneratorInput = generatorInput

		// if the strategy is explicitly Docker, all repos should assume docker
		if g.Strategy == "docker" && input.Uses != nil {
			input.Uses.BuildWithDocker()
		}

		// if we are expecting build inputs, or get a build input when strategy is not docker, expect to build
		if g.ExpectToBuild || (input.ResolvedMatch.Builder && g.Strategy != "docker") {
			input.ExpectToBuild = true
		}

		switch {
		case input.ExpectToBuild && input.ResolvedMatch.IsTemplate():
			// TODO: harder - break the template pieces and check if source code can be attached (look for a build config, build image, etc)
			errs = append(errs, errors.New("template with source code explicitly attached is not supported - you must either specify the template and source code separately or attach an image to the source code using the '[image]~[code]' form"))
			continue
		}
	}
	if len(components) == 0 && g.BinaryBuild && g.Strategy == "source" {
		return nil, errors.New("you must provide a builder image when using the source strategy with a binary build")
	}
	if len(components) == 0 && g.BinaryBuild {
		if len(g.Name) == 0 {
			return nil, errors.New("you must provide a --name when you don't specify a source repository or base image")
		}
		ref := &app.ComponentInput{
			From:          "--binary",
			Argument:      "--binary",
			Value:         g.Name,
			ScratchImage:  true,
			ExpectToBuild: true,
		}
		components = append(components, ref)
	}

	return components, kutilerrors.NewAggregate(errs)
}

// EnsureHasSource ensure every builder component has source code associated with it. It takes a list of component references
// that are builders and have not been associated with source, and a set of source repositories that have not been associated
// with a builder
func EnsureHasSource(components app.ComponentReferences, repositories app.SourceRepositories, g *GenerationInputs) error {
	if len(components) == 0 {
		return nil
	}

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
		glog.V(2).Infof("Using %q as the source for build", repositories[0])
		for _, component := range components {
			glog.V(2).Infof("Pairing with component %v", component)
			component.Input().Use(repositories[0])
			repositories[0].UsedBy(component)
		}

	default:
		switch {
		case g.BinaryBuild && g.ExpectToBuild:
			// create new "fake" binary repos for any component that doesn't already have a repo
			// TODO: source repository should possibly be refactored to be an interface or a type that better reflects
			//   the different types of inputs
			for _, component := range components {
				input := component.Input()
				if input.Uses != nil {
					continue
				}
				repo := app.NewBinarySourceRepository()
				isBuilder := input.ResolvedMatch != nil && input.ResolvedMatch.Builder
				if g.Strategy == "docker" || (len(g.Strategy) == 0 && !isBuilder) {
					repo.BuildWithDocker()
				}
				input.Use(repo)
				repo.UsedBy(input)
				input.ExpectToBuild = true
			}
		case g.ExpectToBuild:
			return errors.New("you must specify at least one source repository URL, provide a Dockerfile, or indicate you wish to use binary builds")
		default:
			for _, component := range components {
				component.Input().ExpectToBuild = false
			}
		}
	}
	return nil
}

// ComponentsForSourceRepositories creates components for repositories that have not been previously associated by a
// builder. These components have already gone through source code detection and have a SourceRepositoryInfo attached
// to them.
func AddMissingComponentsToRefBuilder(
	b *app.ReferenceBuilder, repositories app.SourceRepositories, dockerfileResolver, sourceResolver app.Resolver,
	g *GenerationInputs,
) (app.ComponentReferences, error) {
	errs := []error{}
	result := app.ComponentReferences{}
	for _, repo := range repositories {
		info := repo.Info()
		switch {
		case info == nil:
			errs = append(errs, fmt.Errorf("source not detected for repository %q", repo))
			continue
		case info.Dockerfile != nil && (len(g.Strategy) == 0 || g.Strategy == "docker"):
			node := info.Dockerfile.AST()
			baseImage := dockerfileutil.LastBaseImage(node)
			if baseImage == "" {
				errs = append(errs, fmt.Errorf("the Dockerfile in the repository %q has no FROM instruction", info.Path))
				continue
			}
			refs := b.AddComponents([]string{baseImage}, func(input *app.ComponentInput) app.ComponentReference {
				input.Resolver = dockerfileResolver
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
				input.Resolver = sourceResolver
				input.ExpectToBuild = true
				input.Use(repo)
				repo.UsedBy(input)
				return input
			})
			result = append(result, refs...)
		}
	}
	return result, kutilerrors.NewAggregate(errs)
}
