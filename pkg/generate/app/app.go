package app

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"path"
	"reflect"
	"strconv"
	"strings"

	"code.google.com/p/go-uuid/uuid"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/fsouza/go-dockerclient"

	build "github.com/openshift/origin/pkg/build/api"
	deploy "github.com/openshift/origin/pkg/deploy/api"
	image "github.com/openshift/origin/pkg/image/api"
)

// NameSuggester is an object that can suggest a name for itself
type NameSuggester interface {
	SuggestName() (string, bool)
}

// NameSuggestions suggests names from a collection of NameSuggesters
type NameSuggestions []NameSuggester

// SuggestName suggests a name given a collection of NameSuggesters
func (s NameSuggestions) SuggestName() (string, bool) {
	for i := range s {
		if s[i] == nil {
			continue
		}
		if name, ok := s[i].SuggestName(); ok {
			return name, true
		}
	}
	return "", false
}

// Generated is a list of runtime objects
type Generated struct {
	Items []runtime.Object
}

// WithType extracts a list of runtime objects with the specified type
func (g *Generated) WithType(slicePtr interface{}) bool {
	found := false
	v, err := conversion.EnforcePtr(slicePtr)
	if err != nil || v.Kind() != reflect.Slice {
		// This should not happen at runtime.
		panic("need ptr to slice")
	}
	t := v.Type().Elem()
	for i := range g.Items {
		obj := reflect.ValueOf(g.Items[i]).Elem()
		if !obj.Type().ConvertibleTo(t) {
			continue
		}
		found = true
		v.Set(reflect.Append(v, obj.Convert(t)))
	}
	return found
}

func nameFromGitURL(url *url.URL) (string, bool) {
	// from path
	if len(url.Path) > 0 {
		base := path.Base(url.Path)
		if len(base) > 0 && base != "/" {
			if ext := path.Ext(base); ext == ".git" {
				base = base[:len(base)-4]
			}
			return base, true
		}
	}
	if len(url.Host) > 0 {
		// from host with port
		if host, _, err := net.SplitHostPort(url.Host); err == nil {
			return host, true
		}
		// from host without port
		return url.Host, true
	}
	return "", false
}

// SourceRef is a reference to a build source
type SourceRef struct {
	URL *url.URL
	Ref string
	Dir string
}

// SuggestName returns a name derived from the source URL
func (r *SourceRef) SuggestName() (string, bool) {
	return nameFromGitURL(r.URL)
}

// BuildSource returns an OpenShift BuildSource from the SourceRef
func (r *SourceRef) BuildSource() (*build.BuildSource, []build.BuildTriggerPolicy) {
	return &build.BuildSource{
			Type: build.BuildSourceGit,
			Git: &build.GitBuildSource{
				URI: r.URL.String(),
				Ref: r.Ref,
			},
		}, []build.BuildTriggerPolicy{
			{
				Type: build.GithubWebHookType,
				GithubWebHook: &build.WebHookTrigger{
					Secret: generateSecret(20),
				},
			},
			{
				Type: build.GenericWebHookType,
				GenericWebHook: &build.WebHookTrigger{
					Secret: generateSecret(20),
				},
			},
		}
}

// BuildStrategyRef is a reference to a build strategy
type BuildStrategyRef struct {
	IsDockerBuild bool
	Base          *ImageRef
	DockerContext string
}

// BuildStrategy builds an OpenShift BuildStrategy from a BuildStrategyRef
func (s *BuildStrategyRef) BuildStrategy() (*build.BuildStrategy, []build.BuildTriggerPolicy) {
	if s.IsDockerBuild {
		strategy := &build.BuildStrategy{
			Type: build.DockerBuildStrategyType,
		}
		if len(s.DockerContext) > 0 {
			strategy.DockerStrategy = &build.DockerBuildStrategy{
				ContextDir: s.DockerContext,
			}
		}
		return strategy, s.Base.BuildTriggers()
	}

	return &build.BuildStrategy{
		Type: build.STIBuildStrategyType,
		STIStrategy: &build.STIBuildStrategy{
			Image: s.Base.NameReference(),
		},
	}, nil
}

// ImageRef is a reference to an image
type ImageRef struct {
	Namespace string
	Name      string
	Tag       string
	Registry  string

	AsImageRepository bool

	Repository *image.ImageRepository
	Info       *docker.Image
}

// pullSpec returns the string that can be passed to Docker to fetch this
// image.
func (r *ImageRef) pullSpec() string {
	return image.JoinDockerPullSpec(r.Registry, r.Namespace, r.Name, r.Tag)
}

// NameReference returns the name that other OpenShift objects may refer to this
// image as.  Deployment Configs and Build Configs may look an image up
// in an image repository before creating other objects that use the name.
func (r *ImageRef) NameReference() string {
	if len(r.Registry) == 0 && len(r.Namespace) == 0 {
		if len(r.Tag) != 0 {
			return fmt.Sprintf("%s:%s", r.Name, r.Tag)
		}
		return r.Name
	}
	return r.pullSpec()
}

func (r *ImageRef) RepoName() string {
	name := r.Namespace
	if len(name) > 0 {
		name += "/"
	}
	name += r.Name
	return name
}

func (r *ImageRef) SuggestName() (string, bool) {
	if r == nil || len(r.Name) == 0 {
		return "", false
	}
	return r.Name, true
}

func (r *ImageRef) BuildOutput() *build.BuildOutput {
	if r == nil {
		return &build.BuildOutput{}
	}
	return &build.BuildOutput{
		ImageTag: r.NameReference(),
		Registry: r.Registry,
	}
}

func (r *ImageRef) BuildTriggers() []build.BuildTriggerPolicy {
	// TODO return triggers when image build triggers are available
	return []build.BuildTriggerPolicy{}
}

func (r *ImageRef) ImageRepository() (*image.ImageRepository, error) {
	if r.Repository != nil {
		return r.Repository, nil
	}

	name, ok := r.SuggestName()
	if !ok {
		return nil, fmt.Errorf("unable to suggest an image repository name for %q", r.pullSpec())
	}

	repo := &image.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
	}
	if !r.AsImageRepository {
		repo.DockerImageRepository = r.pullSpec()
	}

	return repo, nil
}

func (r *ImageRef) DeployableContainer() (container *kapi.Container, triggers []deploy.DeploymentTriggerPolicy, err error) {
	if r.Info == nil {
		return nil, nil, fmt.Errorf("image info for %q is required to generate a container definition", r.Name)
	}
	name, ok := r.SuggestName()
	if !ok {
		return nil, nil, fmt.Errorf("unable to suggest a container name for the image %q", r.pullSpec())
	}
	if r.AsImageRepository {
		triggers = []deploy.DeploymentTriggerPolicy{
			{
				Type: deploy.DeploymentTriggerOnImageChange,
				ImageChangeParams: &deploy.DeploymentTriggerImageChangeParams{
					Automatic:      true,
					ContainerNames: []string{name},
					RepositoryName: r.NameReference(),
					Tag:            r.Tag,
				},
			},
		}
	}

	container = &kapi.Container{
		Name:  name,
		Image: r.NameReference(),
	}

	// If imageInfo present, append ports
	if r.Info != nil {
		for p := range r.Info.Config.ExposedPorts {
			port, err := strconv.Atoi(p.Port())
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse port %q: %v", p.Port(), err)
			}
			container.Ports = append(container.Ports, kapi.Port{
				Name:          strings.Join([]string{name, p.Proto(), p.Port()}, "-"),
				ContainerPort: port,
				Protocol:      kapi.Protocol(strings.ToUpper(p.Proto())),
			})
		}
		// TODO: Append volume information and environment variables
	}

	return container, triggers, nil

}

type BuildRef struct {
	Source   *SourceRef
	Input    *ImageRef
	Strategy *BuildStrategyRef
	Output   *ImageRef
}

func (r *BuildRef) BuildConfig() (*build.BuildConfig, error) {
	name, ok := NameSuggestions{r.Source, r.Output}.SuggestName()
	if !ok {
		return nil, fmt.Errorf("unable to suggest a name for this build config from %q", r.Source.URL)
	}
	source := &build.BuildSource{}
	sourceTriggers := []build.BuildTriggerPolicy{}
	if r.Source != nil {
		source, sourceTriggers = r.Source.BuildSource()
	}
	strategy := &build.BuildStrategy{}
	strategyTriggers := []build.BuildTriggerPolicy{}
	if r.Strategy != nil {
		strategy, strategyTriggers = r.Strategy.BuildStrategy()
	}
	return &build.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Triggers: append(sourceTriggers, strategyTriggers...),
		Parameters: build.BuildParameters{
			Source:   *source,
			Strategy: *strategy,
			Output:   *r.Output.BuildOutput(),
		},
	}, nil
}

type DeploymentConfigRef struct {
	Images []*ImageRef
	Env    Environment
}

// TODO: take a pod template spec as argument
func (r *DeploymentConfigRef) DeploymentConfig() (*deploy.DeploymentConfig, error) {
	suggestions := NameSuggestions{}
	for i := range r.Images {
		suggestions = append(suggestions, r.Images[i])
	}
	name, ok := suggestions.SuggestName()
	if !ok {
		return nil, fmt.Errorf("unable to suggest a name for this deployment config")
	}

	selector := map[string]string{
		"deploymentconfig": name,
	}

	triggers := []deploy.DeploymentTriggerPolicy{
		// By default, always deploy on change
		{
			Type: deploy.DeploymentTriggerOnConfigChange,
		},
	}

	template := kapi.PodSpec{}
	for i := range r.Images {
		c, containerTriggers, err := r.Images[i].DeployableContainer()
		if err != nil {
			return nil, err
		}
		triggers = append(triggers, containerTriggers...)
		template.Containers = append(template.Containers, *c)
	}
	// TODO: populate volumes

	for i := range template.Containers {
		template.Containers[i].Env = append(template.Containers[i].Env, r.Env.List()...)
	}

	return &deploy.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Template: deploy.DeploymentTemplate{
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Replicas: 1,
				Selector: selector,
				Template: &kapi.PodTemplateSpec{
					ObjectMeta: kapi.ObjectMeta{
						Labels: selector,
					},
					Spec: template,
				},
			},
		},
		Triggers: triggers,
	}, nil
}

// generateSecret generates a random secret string
func generateSecret(n int) string {
	n = n * 3 / 4
	b := make([]byte, n)
	read, _ := rand.Read(b)
	if read != n {
		return uuid.NewRandom().String()
	}
	return base64.URLEncoding.EncodeToString(b)
}
