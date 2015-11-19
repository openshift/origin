package app

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"code.google.com/p/go-uuid/uuid"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/conversion"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/generate/git"
	"github.com/openshift/origin/pkg/util"
)

const (
	volumeNameInfix = "volume"
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
	if url == nil {
		return "", false
	}
	// from path
	if name, ok := git.NameFromRepositoryURL(url); ok {
		return name, true
	}
	// TODO: path is questionable
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

	DockerfileContents string

	Binary bool
}

func urlWithoutRef(url url.URL) string {
	url.Fragment = ""
	return url.String()
}

// SuggestName returns a name derived from the source URL
func (r *SourceRef) SuggestName() (string, bool) {
	if r == nil {
		return "", false
	}
	if len(r.Name) > 0 {
		return r.Name, true
	}
	return nameFromGitURL(r.URL)
}

// BuildSource returns an OpenShift BuildSource from the SourceRef
func (r *SourceRef) BuildSource() (*buildapi.BuildSource, []buildapi.BuildTriggerPolicy) {
	triggers := []buildapi.BuildTriggerPolicy{
		{
			Type: buildapi.GitHubWebHookBuildTriggerType,
			GitHubWebHook: &buildapi.WebHookTrigger{
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
	source := &buildapi.BuildSource{}

	if len(r.DockerfileContents) != 0 {
		source.Type = buildapi.BuildSourceDockerfile
		source.Dockerfile = &r.DockerfileContents
	}
	if r.URL != nil {
		source.Type = buildapi.BuildSourceGit
		source.Git = &buildapi.GitBuildSource{
			URI: urlWithoutRef(*r.URL),
			Ref: r.Ref,
		}
		source.ContextDir = r.ContextDir
	}
	if r.Binary {
		source.Type = buildapi.BuildSourceBinary
		source.Binary = &buildapi.BinaryBuildSource{}
	}
	return source, triggers
}

// BuildStrategyRef is a reference to a build strategy
type BuildStrategyRef struct {
	IsDockerBuild bool
	Base          *ImageRef
}

// BuildStrategy builds an OpenShift BuildStrategy from a BuildStrategyRef
func (s *BuildStrategyRef) BuildStrategy(env Environment) (*buildapi.BuildStrategy, []buildapi.BuildTriggerPolicy) {
	if s.IsDockerBuild {
		dockerFrom := s.Base.ObjectReference()
		return &buildapi.BuildStrategy{
			Type: buildapi.DockerBuildStrategyType,
			DockerStrategy: &buildapi.DockerBuildStrategy{
				From: &dockerFrom,
				Env:  env.List(),
			},
		}, s.Base.BuildTriggers()
	}

	return &buildapi.BuildStrategy{
		Type: buildapi.SourceBuildStrategyType,
		SourceStrategy: &buildapi.SourceBuildStrategy{
			From: s.Base.ObjectReference(),
			Env:  env.List(),
		},
	}, s.Base.BuildTriggers()
}

// BuildRef is a reference to a build configuration
type BuildRef struct {
	Source   *SourceRef
	Input    *ImageRef
	Strategy *BuildStrategyRef
	Output   *ImageRef
	Env      Environment
}

// BuildConfig creates a buildConfig resource from the build configuration reference
func (r *BuildRef) BuildConfig() (*buildapi.BuildConfig, error) {
	name, ok := NameSuggestions{r.Source, r.Output}.SuggestName()
	if !ok {
		return nil, fmt.Errorf("unable to suggest a name for this BuildConfig from %q", r.Source.URL)
	}
	var source *buildapi.BuildSource
	triggers := []buildapi.BuildTriggerPolicy{}
	if r.Source != nil {
		source, triggers = r.Source.BuildSource()
	}
	if source == nil {
		source = &buildapi.BuildSource{}
	}
	strategy := &buildapi.BuildStrategy{}
	strategyTriggers := []buildapi.BuildTriggerPolicy{}
	if r.Strategy != nil {
		strategy, strategyTriggers = r.Strategy.BuildStrategy(r.Env)
	}
	output, err := r.Output.BuildOutput()
	if err != nil {
		return nil, err
	}

	if source.Binary == nil {
		configChangeTrigger := buildapi.BuildTriggerPolicy{
			Type: buildapi.ConfigChangeBuildTriggerType,
		}
		triggers = append(triggers, configChangeTrigger)
		triggers = append(triggers, strategyTriggers...)
	}

	return &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Spec: buildapi.BuildConfigSpec{
			Triggers: triggers,
			BuildSpec: buildapi.BuildSpec{
				Source:   *source,
				Strategy: *strategy,
				Output:   *output,
			},
		},
	}, nil
}

// DeploymentConfigRef is a reference to a deployment configuration
type DeploymentConfigRef struct {
	Name   string
	Images []*ImageRef
	Env    Environment
	Labels map[string]string
}

// DeploymentConfig creates a deploymentConfig resource from the deployment configuration reference
//
// TODO: take a pod template spec as argument
func (r *DeploymentConfigRef) DeploymentConfig() (*deployapi.DeploymentConfig, error) {
	if len(r.Name) == 0 {
		suggestions := NameSuggestions{}
		for i := range r.Images {
			suggestions = append(suggestions, r.Images[i])
		}
		name, ok := suggestions.SuggestName()
		if !ok {
			return nil, fmt.Errorf("unable to suggest a name for this DeploymentConfig")
		}
		r.Name = name
	}

	selector := map[string]string{
		"deploymentconfig": r.Name,
	}
	if len(r.Labels) > 0 {
		if err := util.MergeInto(selector, r.Labels, 0); err != nil {
			return nil, err
		}
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

	// Create EmptyDir volumes for all container volume mounts
	for _, c := range template.Containers {
		for _, v := range c.VolumeMounts {
			template.Volumes = append(template.Volumes, kapi.Volume{
				Name: v.Name,
				VolumeSource: kapi.VolumeSource{
					EmptyDir: &kapi.EmptyDirVolumeSource{Medium: kapi.StorageMediumDefault},
				},
			})
		}
	}

	for i := range template.Containers {
		template.Containers[i].Env = append(template.Containers[i].Env, r.Env.List()...)
	}

	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: r.Name,
		},
		Template: deployapi.DeploymentTemplate{
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
			return nil, nil, fmt.Errorf("unknown label spec: %s", labelSpec)
		}
	}
	for _, removeLabel := range remove {
		if _, found := labels[removeLabel]; found {
			return nil, nil, fmt.Errorf("can not both modify and remove a label in the same command")
		}
	}
	return labels, remove, nil
}
