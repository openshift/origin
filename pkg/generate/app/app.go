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

	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
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
	URL        *url.URL
	Ref        string
	Dir        string
	Name       string
	ContextDir string
}

func urlWithoutRef(url url.URL) string {
	url.Fragment = ""
	return url.String()
}

// SuggestName returns a name derived from the source URL
func (r *SourceRef) SuggestName() (string, bool) {
	if len(r.Name) > 0 {
		return r.Name, true
	}
	return nameFromGitURL(r.URL)
}

// BuildSource returns an OpenShift BuildSource from the SourceRef
func (r *SourceRef) BuildSource() (*buildapi.BuildSource, []buildapi.BuildTriggerPolicy) {
	return &buildapi.BuildSource{
			Type: buildapi.BuildSourceGit,
			Git: &buildapi.GitBuildSource{
				URI: urlWithoutRef(*r.URL),
				Ref: r.Ref,
			},
			ContextDir: r.ContextDir,
		}, []buildapi.BuildTriggerPolicy{
			{
				Type: buildapi.GithubWebHookBuildTriggerType,
				GithubWebHook: &buildapi.WebHookTrigger{
					Secret: generateSecret(20),
				},
			},
			{
				Type: buildapi.GenericWebHookBuildTriggerType,
				GenericWebHook: &buildapi.WebHookTrigger{
					Secret: generateSecret(20),
				},
			},
		}
}

// BuildStrategyRef is a reference to a build strategy
type BuildStrategyRef struct {
	IsDockerBuild bool
	Base          *ImageRef
}

// BuildStrategy builds an OpenShift BuildStrategy from a BuildStrategyRef
func (s *BuildStrategyRef) BuildStrategy() (*buildapi.BuildStrategy, []buildapi.BuildTriggerPolicy) {
	if s.IsDockerBuild {
		strategy := &buildapi.BuildStrategy{
			Type: buildapi.DockerBuildStrategyType,
		}
		return strategy, s.Base.BuildTriggers()
	}

	return &buildapi.BuildStrategy{
		Type: buildapi.STIBuildStrategyType,
		STIStrategy: &buildapi.STIBuildStrategy{
			Image: s.Base.String(),
		},
	}, nil
}

// ImageRef is a reference to an image
type ImageRef struct {
	imageapi.DockerImageReference
	AsImageRepository bool

	Repository *imageapi.ImageRepository
	Info       *imageapi.DockerImage
}

/*
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
*/

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

func (r *ImageRef) BuildOutput() (*buildapi.BuildOutput, error) {
	if r == nil {
		return &buildapi.BuildOutput{}, nil
	}
	imageRepo, err := r.ImageRepository()
	if err != nil {
		return nil, err
	}
	return &buildapi.BuildOutput{
		To: &kapi.ObjectReference{
			Name: imageRepo.Name,
		},
	}, nil
}

func (r *ImageRef) BuildTriggers() []buildapi.BuildTriggerPolicy {
	// TODO return triggers when image build triggers are available
	return []buildapi.BuildTriggerPolicy{}
}

func (r *ImageRef) ImageRepository() (*imageapi.ImageRepository, error) {
	if r.Repository != nil {
		return r.Repository, nil
	}

	name, ok := r.SuggestName()
	if !ok {
		return nil, fmt.Errorf("unable to suggest an image repository name for %q", r.String())
	}

	repo := &imageapi.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
	}
	if !r.AsImageRepository {
		repo.DockerImageRepository = r.String()
	}

	return repo, nil
}

func (r *ImageRef) DeployableContainer() (container *kapi.Container, triggers []deployapi.DeploymentTriggerPolicy, err error) {
	name, ok := r.SuggestName()
	if !ok {
		return nil, nil, fmt.Errorf("unable to suggest a container name for the image %q", r.String())
	}
	if r.AsImageRepository {
		tag := r.Tag
		if len(tag) == 0 {
			tag = "latest"
		}
		triggers = []deployapi.DeploymentTriggerPolicy{
			{
				Type: deployapi.DeploymentTriggerOnImageChange,
				ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
					Automatic:      true,
					ContainerNames: []string{name},
					From: kapi.ObjectReference{
						Name: name,
					},
					Tag: tag,
				},
			},
		}
	}

	container = &kapi.Container{
		Name:  name,
		Image: r.String(),
	}

	// If imageInfo present, append ports
	if r.Info != nil {
		ports := []string{}
		// ExposedPorts can consist of multiple space-separated ports
		for exposed := range r.Info.Config.ExposedPorts {
			ports = append(ports, strings.Split(exposed, " ")...)
		}

		for _, sp := range ports {
			p := docker.Port(sp)
			port, err := strconv.Atoi(p.Port())
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse port %q: %v", p.Port(), err)
			}
			container.Ports = append(container.Ports, kapi.ContainerPort{
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

func (r *BuildRef) BuildConfig() (*buildapi.BuildConfig, error) {
	name, ok := NameSuggestions{r.Source, r.Output}.SuggestName()
	if !ok {
		return nil, fmt.Errorf("unable to suggest a name for this build config from %q", r.Source.URL)
	}
	source := &buildapi.BuildSource{}
	sourceTriggers := []buildapi.BuildTriggerPolicy{}
	if r.Source != nil {
		source, sourceTriggers = r.Source.BuildSource()
	}
	strategy := &buildapi.BuildStrategy{}
	strategyTriggers := []buildapi.BuildTriggerPolicy{}
	if r.Strategy != nil {
		strategy, strategyTriggers = r.Strategy.BuildStrategy()
	}
	output, err := r.Output.BuildOutput()
	if err != nil {
		return nil, err
	}
	return &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Triggers: append(sourceTriggers, strategyTriggers...),
		Parameters: buildapi.BuildParameters{
			Source:   *source,
			Strategy: *strategy,
			Output:   *output,
		},
	}, nil
}

type DeploymentConfigRef struct {
	Images []*ImageRef
	Env    Environment
}

// TODO: take a pod template spec as argument
func (r *DeploymentConfigRef) DeploymentConfig() (*deployapi.DeploymentConfig, error) {
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

	triggers := []deployapi.DeploymentTriggerPolicy{
		// By default, always deploy on change
		{
			Type: deployapi.DeploymentTriggerOnConfigChange,
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

	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Template: deployapi.DeploymentTemplate{
			Strategy: deployapi.DeploymentStrategy{
				Type: deployapi.DeploymentStrategyTypeRecreate,
			},
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

// ContainerPortsFromString extracts sets of port specifications from a comma-delimited string. Each segment
// must be a single port number (container port) or a colon delimited pair of ports (container port and host port).
func ContainerPortsFromString(portString string) ([]kapi.ContainerPort, error) {
	ports := []kapi.ContainerPort{}
	for _, s := range strings.Split(portString, ",") {
		port, ok := checkPortSpecSegment(s)
		if !ok {
			return nil, fmt.Errorf("%q is not valid: you must specify one (container) or two (container:host) port numbers", s)
		}
		ports = append(ports, port)
	}
	return ports, nil
}

func checkPortSpecSegment(s string) (port kapi.ContainerPort, ok bool) {
	if strings.Contains(s, ":") {
		pair := strings.Split(s, ":")
		if len(pair) != 2 {
			return
		}
		container, err := strconv.Atoi(pair[0])
		if err != nil {
			return
		}
		host, err := strconv.Atoi(pair[1])
		if err != nil {
			return
		}
		return kapi.ContainerPort{ContainerPort: container, HostPort: host}, true
	}

	container, err := strconv.Atoi(s)
	if err != nil {
		return
	}
	return kapi.ContainerPort{ContainerPort: container}, true
}

// LabelsFromSpec turns a set of specs NAME=VALUE or NAME- into a map of labels,
// a remove label list, or an error.
func LabelsFromSpec(spec []string) (map[string]string, []string, error) {
	labels := map[string]string{}
	var remove []string
	for _, labelSpec := range spec {
		if strings.Index(labelSpec, "=") != -1 {
			parts := strings.Split(labelSpec, "=")
			if len(parts) != 2 {
				return nil, nil, fmt.Errorf("invalid label spec: %v", labelSpec)
			}
			labels[parts[0]] = parts[1]
		} else if strings.HasSuffix(labelSpec, "-") {
			remove = append(remove, labelSpec[:len(labelSpec)-1])
		} else {
			return nil, nil, fmt.Errorf("unknown label spec: %v")
		}
	}
	for _, removeLabel := range remove {
		if _, found := labels[removeLabel]; found {
			return nil, nil, fmt.Errorf("can not both modify and remove a label in the same command")
		}
	}
	return labels, remove, nil
}
