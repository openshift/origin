package app

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/pborman/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/git"
	"github.com/openshift/origin/pkg/oc/generate"
	"github.com/openshift/origin/pkg/util"

	s2igit "github.com/openshift/source-to-image/pkg/scm/git"
)

const (
	volumeNameInfix = "volume"

	GenerationWarningAnnotation = "app.generate.openshift.io/warnings"
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

// IsParameterizableValue returns true if the value contains standard replacement
// syntax, to preserve the value for use inside of the generated output. Passing
// parameters into output is only valid if the output is used inside of a template.
func IsParameterizableValue(s string) bool {
	return strings.Contains(s, "${") || strings.Contains(s, "$(")
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

func nameFromGitURL(url *s2igit.URL) (string, bool) {
	if url == nil {
		return "", false
	}
	// from path
	if name, ok := git.NameFromRepositoryURL(&url.URL); ok {
		return name, true
	}
	// TODO: path is questionable
	if len(url.URL.Host) > 0 {
		// from host with port
		if host, _, err := net.SplitHostPort(url.URL.Host); err == nil {
			return host, true
		}
		// from host without port
		return url.URL.Host, true
	}
	return "", false
}

// SourceRef is a reference to a build source
type SourceRef struct {
	URL        *s2igit.URL
	Dir        string
	Name       string
	ContextDir string
	Secrets    []buildapi.SecretBuildSource

	SourceImage     *ImageRef
	ImageSourcePath string
	ImageDestPath   string

	DockerfileContents string

	Binary bool

	RequiresAuth bool
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
				Secret: GenerateSecret(20),
			},
		},
		{
			Type: buildapi.GenericWebHookBuildTriggerType,
			GenericWebHook: &buildapi.WebHookTrigger{
				Secret: GenerateSecret(20),
			},
		},
	}
	source := &buildapi.BuildSource{}
	source.Secrets = r.Secrets

	if len(r.DockerfileContents) != 0 {
		source.Dockerfile = &r.DockerfileContents
	}
	if r.URL != nil {
		source.Git = &buildapi.GitBuildSource{
			URI: r.URL.StringNoFragment(),
			Ref: r.URL.URL.Fragment,
		}
		source.ContextDir = r.ContextDir
	}
	if r.Binary {
		source.Binary = &buildapi.BinaryBuildSource{}
	}
	if r.SourceImage != nil {
		objRef := r.SourceImage.ObjectReference()
		imgSrc := buildapi.ImageSource{}
		imgSrc.From = objRef
		imgSrc.Paths = []buildapi.ImageSourcePath{
			{
				SourcePath:     r.ImageSourcePath,
				DestinationDir: r.ImageDestPath,
			},
		}
		triggers = append(triggers, buildapi.BuildTriggerPolicy{
			Type: buildapi.ImageChangeBuildTriggerType,
			ImageChange: &buildapi.ImageChangeTrigger{
				From: &objRef,
			},
		})
		source.Images = []buildapi.ImageSource{imgSrc}
	}
	return source, triggers
}

// BuildStrategyRef is a reference to a build strategy
type BuildStrategyRef struct {
	Strategy generate.Strategy
	Base     *ImageRef
}

// BuildStrategy builds an OpenShift BuildStrategy from a BuildStrategyRef
func (s *BuildStrategyRef) BuildStrategy(env Environment, dockerStrategyOptions *buildapi.DockerStrategyOptions) (*buildapi.BuildStrategy, []buildapi.BuildTriggerPolicy) {
	switch s.Strategy {
	case generate.StrategyPipeline:
		return &buildapi.BuildStrategy{
			JenkinsPipelineStrategy: &buildapi.JenkinsPipelineBuildStrategy{},
		}, s.Base.BuildTriggers()

	case generate.StrategyDocker:
		var triggers []buildapi.BuildTriggerPolicy
		strategy := &buildapi.DockerBuildStrategy{
			Env: env.List(),
		}
		if dockerStrategyOptions != nil {
			strategy.BuildArgs = dockerStrategyOptions.BuildArgs
		}
		if s.Base != nil {
			ref := s.Base.ObjectReference()
			strategy.From = &ref
			triggers = s.Base.BuildTriggers()
		}
		return &buildapi.BuildStrategy{
			DockerStrategy: strategy,
		}, triggers

	case generate.StrategySource:
		return &buildapi.BuildStrategy{
			SourceStrategy: &buildapi.SourceBuildStrategy{
				From: s.Base.ObjectReference(),
				Env:  env.List(),
			},
		}, s.Base.BuildTriggers()
	}

	glog.Error("BuildStrategy called with unknown strategy")
	return nil, nil
}

// BuildRef is a reference to a build configuration
type BuildRef struct {
	Source                *SourceRef
	Input                 *ImageRef
	Strategy              *BuildStrategyRef
	DockerStrategyOptions *buildapi.DockerStrategyOptions
	Output                *ImageRef
	Env                   Environment
	Binary                bool
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
		strategy, strategyTriggers = r.Strategy.BuildStrategy(r.Env, r.DockerStrategyOptions)
	}
	output, err := r.Output.BuildOutput()
	if err != nil {
		return nil, err
	}

	if !r.Binary {
		configChangeTrigger := buildapi.BuildTriggerPolicy{
			Type: buildapi.ConfigChangeBuildTriggerType,
		}
		triggers = append(triggers, configChangeTrigger)
		triggers = append(triggers, strategyTriggers...)
	} else {
		// remove imagechangetriggers from binary buildconfigs because
		// triggered builds will fail (no binary input available)
		filteredTriggers := []buildapi.BuildTriggerPolicy{}
		for _, trigger := range triggers {
			if trigger.Type != buildapi.ImageChangeBuildTriggerType {
				filteredTriggers = append(filteredTriggers, trigger)
			}
		}
		triggers = filteredTriggers
	}
	return &buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: buildapi.BuildConfigSpec{
			Triggers: triggers,
			CommonSpec: buildapi.CommonSpec{
				Source:   *source,
				Strategy: *strategy,
				Output:   *output,
			},
		},
	}, nil
}

type DeploymentHook struct {
	Shell string
}

// DeploymentConfigRef is a reference to a deployment configuration
type DeploymentConfigRef struct {
	Name     string
	Images   []*ImageRef
	Env      Environment
	Labels   map[string]string
	AsTest   bool
	PostHook *DeploymentHook
}

// DeploymentConfig creates a deploymentConfig resource from the deployment configuration reference
//
// TODO: take a pod template spec as argument
func (r *DeploymentConfigRef) DeploymentConfig() (*appsapi.DeploymentConfig, error) {
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

	triggers := []appsapi.DeploymentTriggerPolicy{
		// By default, always deploy on change
		{
			Type: appsapi.DeploymentTriggerOnConfigChange,
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

	dc := &appsapi.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: r.Name,
		},
		Spec: appsapi.DeploymentConfigSpec{
			Replicas: 1,
			Test:     r.AsTest,
			Selector: selector,
			Template: &kapi.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: selector,
				},
				Spec: template,
			},
			Triggers: triggers,
		},
	}
	if r.PostHook != nil {
		//dc.Spec.Strategy.Type = "Rolling"
		if len(r.PostHook.Shell) > 0 {
			dc.Spec.Strategy.RecreateParams = &appsapi.RecreateDeploymentStrategyParams{
				Post: &appsapi.LifecycleHook{
					ExecNewPod: &appsapi.ExecNewPodHook{
						Command: []string{"/bin/sh", "-c", r.PostHook.Shell},
					},
				},
			}
		}
	}

	return dc, nil
}

// GenerateSecret generates a random secret string
func GenerateSecret(n int) string {
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
		return kapi.ContainerPort{ContainerPort: int32(container), HostPort: int32(host)}, true
	}

	container, err := strconv.Atoi(s)
	if err != nil {
		return
	}
	return kapi.ContainerPort{ContainerPort: int32(container)}, true
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

// TODO: move to pkg/runtime or pkg/api
func AsVersionedObjects(objects []runtime.Object, typer runtime.ObjectTyper, convertor runtime.ObjectConvertor, versions ...schema.GroupVersion) []error {
	var errs []error
	for i, object := range objects {
		kinds, _, err := typer.ObjectKinds(object)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if kindsInVersions(kinds, versions) {
			continue
		}
		if !isInternalOnly(kinds) {
			continue
		}
		converted, err := tryConvert(convertor, object, versions)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		objects[i] = converted
	}
	return errs
}

func isInternalOnly(kinds []schema.GroupVersionKind) bool {
	for _, kind := range kinds {
		if kind.Version != runtime.APIVersionInternal {
			return false
		}
	}
	return true
}

func kindsInVersions(kinds []schema.GroupVersionKind, versions []schema.GroupVersion) bool {
	for _, kind := range kinds {
		for _, version := range versions {
			if kind.GroupVersion() == version {
				return true
			}
		}
	}
	return false
}

// tryConvert attempts to convert the given object to the provided versions in order.
func tryConvert(convertor runtime.ObjectConvertor, object runtime.Object, versions []schema.GroupVersion) (runtime.Object, error) {
	var last error
	for _, version := range versions {
		obj, err := convertor.ConvertToVersion(object, version)
		if err != nil {
			last = err
			continue
		}
		return obj, nil
	}
	return nil, last
}
