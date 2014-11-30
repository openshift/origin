package app

import (
	"fmt"
	"net"
	"net/url"
	"path"
	"reflect"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	//"github.com/fsouza/go-dockerclient"

	build "github.com/openshift/origin/pkg/build/api"
	deploy "github.com/openshift/origin/pkg/deploy/api"
	image "github.com/openshift/origin/pkg/image/api"
)

type NameSuggester interface {
	SuggestName() (string, bool)
}

type NameSuggestions []NameSuggester

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

type Generated struct {
	Items []runtime.Object
}

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

type SourceRef struct {
	URL *url.URL
	Ref string
	// TODO: pointer to a built in source repository
}

func SourceRefForGitURL(location string) (*SourceRef, error) {
	url, err := url.Parse(location)
	if err != nil {
		return nil, err
	}

	ref := url.Fragment
	url.Fragment = ""
	if len(ref) == 0 {
		ref = "master"
	}

	return &SourceRef{URL: url, Ref: ref}, nil
}

func (r *SourceRef) SuggestName() (string, bool) {
	return nameFromGitURL(r.URL)
}

func (r *SourceRef) BuildSource() (*build.BuildSource, []build.BuildTriggerPolicy) {
	return &build.BuildSource{
		Type: build.BuildSourceGit,
		Git: &build.GitBuildSource{
			URI: r.URL.String(),
			Ref: r.Ref,
		},
	}, nil
}

type ImageRef struct {
	Namespace string
	Name      string
	Tag       string
	Registry  string

	AsImageRepository bool

	repository *image.ImageRepository
}

func ImageRefForImageRepository(repo *image.ImageRepository, tag string) (*ImageRef, error) {
	pullSpec := repo.Status.DockerImageRepository
	if len(pullSpec) == 0 {
		// need to know the default OpenShift registry
		return nil, fmt.Errorf("the repository does not resolve to a pullable Docker repository")
	}
	registry, namespace, name, repoTag, err := image.SplitDockerPullSpec(pullSpec)
	if err != nil {
		return nil, err
	}

	if len(tag) == 0 {
		if len(repoTag) != 0 {
			tag = repoTag
		} else {
			tag = "latest"
		}
	}

	return &ImageRef{
		Registry:  registry,
		Namespace: namespace,
		Name:      name,
		Tag:       tag,

		repository: repo,
	}, nil
}

// pullSpec returns the string that can be passed to Docker to fetch this
// image.
func (r *ImageRef) pullSpec() string {
	return image.JoinDockerPullSpec(r.Registry, r.Namespace, r.Name, r.Tag)
}

// nameReference returns the name that other OpenShift objects may refer to this
// image as.  Deployment Configs and Build Configs may look an image up
// in an image repository before creating other objects that use the name.
func (r *ImageRef) nameReference() string {
	if len(r.Registry) == 0 && len(r.Namespace) == 0 {
		if len(r.Tag) != 0 {
			return fmt.Sprintf("%s:%s", r.Name, r.Tag)
		}
		return r.Name
	}
	return r.pullSpec()
}

func (r *ImageRef) SuggestName() (string, bool) {
	if r == nil || len(r.Name) == 0 {
		return "", false
	}
	return r.Name, true
}

func (r *ImageRef) BuildStrategy() (*build.BuildStrategy, []build.BuildTriggerPolicy) {
	//TODO: handle this being an STI image
	return &build.BuildStrategy{
		Type: build.DockerBuildStrategyType,
	}, nil
}

func (r *ImageRef) BuildOutput() *build.BuildOutput {
	if r == nil {
		return &build.BuildOutput{}
	}
	return &build.BuildOutput{
		ImageTag: r.nameReference(),
		Registry: r.Registry,
	}
}

func (r *ImageRef) ImageRepository() (*image.ImageRepository, error) {
	if r.repository != nil {
		return r.repository, nil
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

func (r *ImageRef) DeployableContainer() (*kapi.Container, []deploy.DeploymentTriggerPolicy, error) {
	name, ok := r.SuggestName()
	if !ok {
		return nil, nil, fmt.Errorf("unable to suggest a container name for the image %q", r.pullSpec())
	}
	triggers := []deploy.DeploymentTriggerPolicy{
		{
			Type: deploy.DeploymentTriggerOnImageChange,
			ImageChangeParams: &deploy.DeploymentTriggerImageChangeParams{
				Automatic:      true,
				ContainerNames: []string{name},
				RepositoryName: r.nameReference(),
				Tag:            r.Tag,
			},
		},
	}
	return &kapi.Container{
		Name:  name,
		Image: r.nameReference(),
		// TODO: populate the remainder of these fields with something like podex
	}, triggers, nil
}

type BuildRef struct {
	Source *SourceRef
	Base   *ImageRef
	Output *ImageRef
}

func (r *BuildRef) BuildConfig() (*build.BuildConfig, error) {
	name, ok := NameSuggestions{r.Source, r.Output, r.Base}.SuggestName()
	if !ok {
		return nil, fmt.Errorf("unable to suggest a name for this build config from %q", r.Source.URL)
	}
	source, sourceTriggers := r.Source.BuildSource()
	strategy, strategyTriggers := r.Base.BuildStrategy()
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
}

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

	template := kapi.ContainerManifest{}
	for i := range r.Images {
		c, containerTriggers, err := r.Images[i].DeployableContainer()
		if err != nil {
			return nil, err
		}
		triggers = append(triggers, containerTriggers...)
		// TODO: populate environment
		template.Containers = append(template.Containers, *c)
	}
	// TODO: populate volumes

	return &deploy.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Template: deploy.DeploymentTemplate{
			ControllerTemplate: kapi.ReplicationControllerState{
				Replicas:        1,
				ReplicaSelector: selector,
				PodTemplate: kapi.PodTemplate{
					Labels: selector,
					DesiredState: kapi.PodState{
						Manifest: template,
					},
				},
			},
		},
		Triggers: triggers,
	}, nil
}
