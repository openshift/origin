package defaults

import (
	"fmt"
	"io"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	buildadmission "github.com/openshift/origin/pkg/build/admission"
	defaultsapi "github.com/openshift/origin/pkg/build/admission/defaults/api"
	"github.com/openshift/origin/pkg/build/admission/defaults/api/validation"
	buildapi "github.com/openshift/origin/pkg/build/api"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	projectcache "github.com/openshift/origin/pkg/project/cache"
)

func init() {
	admission.RegisterPlugin("BuildDefaults", func(c clientset.Interface, config io.Reader) (admission.Interface, error) {

		defaultsConfig, err := getConfig(config)
		if err != nil {
			return nil, err
		}

		glog.V(5).Infof("Initializing BuildDefaults plugin with config: %#v", defaultsConfig)
		return NewBuildDefaults(defaultsConfig, c), nil
	})
}

func getConfig(in io.Reader) (*defaultsapi.BuildDefaultsConfig, error) {
	defaultsConfig := &defaultsapi.BuildDefaultsConfig{}
	err := buildadmission.ReadPluginConfig(in, defaultsConfig)
	if err != nil {
		return nil, err
	}
	errs := validation.ValidateBuildDefaultsConfig(defaultsConfig)
	if len(errs) > 0 {
		return nil, errs.ToAggregate()
	}
	return defaultsConfig, nil
}

type buildDefaults struct {
	*admission.Handler
	defaultsConfig *defaultsapi.BuildDefaultsConfig
	cache          *projectcache.ProjectCache
	client         clientset.Interface
}

// NewBuildDefaults returns an admission control for builds that sets build defaults
// based on the plugin configuration
func NewBuildDefaults(defaultsConfig *defaultsapi.BuildDefaultsConfig, c clientset.Interface) admission.Interface {
	return &buildDefaults{
		Handler:        admission.NewHandler(admission.Create),
		defaultsConfig: defaultsConfig,
		client:         c,
	}
}

var _ = oadmission.WantsProjectCache(&buildDefaults{})

func (a *buildDefaults) SetProjectCache(cache *projectcache.ProjectCache) {
	a.cache = cache
}

// Admit applies configured build defaults to a pod that is identified
// as a build pod.
func (a *buildDefaults) Admit(attributes admission.Attributes) error {
	if a.defaultsConfig == nil {
		return nil
	}
	if !buildadmission.IsBuildPod(attributes) {
		return nil
	}
	build, version, err := buildadmission.GetBuild(attributes)
	if err != nil {
		return nil
	}

	glog.V(4).Infof("Handling build %s/%s", build.Namespace, build.Name)

	a.applyBuildDefaults(build)

	err = buildadmission.SetBuildLogLevel(attributes, build)
	if err != nil {
		return err
	}

	return buildadmission.SetBuild(attributes, build, version)
}

func (a *buildDefaults) applyBuildDefaults(build *buildapi.Build) {
	// Apply default env
	buildEnv := getBuildEnv(build)
	for _, envVar := range a.defaultsConfig.Env {
		glog.V(5).Infof("Adding default environment variable %s=%s to build %s/%s", envVar.Name, envVar.Value, build.Namespace, build.Name)
		addDefaultEnvVar(envVar, buildEnv)
	}

	// Apply default labels
	for _, lbl := range a.defaultsConfig.ImageLabels {
		glog.V(5).Infof("Adding default image label %s=%s to build %s/%s", lbl.Name, lbl.Value, build.Namespace, build.Name)
		addDefaultLabel(lbl, &build.Spec.Output.ImageLabels)
	}

	sourceDefaults := a.defaultsConfig.SourceStrategyDefaults
	sourceStrategy := build.Spec.Strategy.SourceStrategy
	if sourceDefaults != nil && sourceDefaults.Incremental != nil && *sourceDefaults.Incremental &&
		sourceStrategy != nil && sourceStrategy.Incremental == nil {
		glog.V(5).Infof("Setting source strategy Incremental to true in build %s/%s", build.Namespace, build.Name)
		t := true
		build.Spec.Strategy.SourceStrategy.Incremental = &t
	}

	// Apply git proxy defaults
	if build.Spec.Source.Git == nil {
		return
	}
	if len(a.defaultsConfig.GitHTTPProxy) != 0 {
		if build.Spec.Source.Git.HTTPProxy == nil {
			t := a.defaultsConfig.GitHTTPProxy
			glog.V(5).Infof("Setting default Git HTTP proxy of build %s/%s to %s", build.Namespace, build.Name, t)
			build.Spec.Source.Git.HTTPProxy = &t
		}
	}

	if len(a.defaultsConfig.GitHTTPSProxy) != 0 {
		if build.Spec.Source.Git.HTTPSProxy == nil {
			t := a.defaultsConfig.GitHTTPSProxy
			glog.V(5).Infof("Setting default Git HTTPS proxy of build %s/%s to %s", build.Namespace, build.Name, t)
			build.Spec.Source.Git.HTTPSProxy = &t
		}
	}

	if len(a.defaultsConfig.GitNoProxy) != 0 {
		if build.Spec.Source.Git.NoProxy == nil {
			t := a.defaultsConfig.GitNoProxy
			glog.V(5).Infof("Setting default Git no proxy of build %s/%s to %s", build.Namespace, build.Name, t)
			build.Spec.Source.Git.NoProxy = &t
		}
	}

	//apply default source secret if one set after all validation
	//BUG: build fails because secret is not mounted...
	secret, err := a.setDefaultSourceSecret(build)
	if err == nil {
		if build.Spec.Source.SourceSecret == nil {
			//check if secret exist for Setting
			_, err := a.client.Core().Secrets(build.Namespace).Get(secret)
			if err != nil {
				glog.V(5).Infof("Default sourceSecret %s not found for  %s/%s  . Skipping it. ", build.Namespace, build.Name, secret)
			} else {
				glog.V(5).Infof("Setting sourceSecret for %s/%s %s ", build.Namespace, build.Name, secret)
				var ss kapi.LocalObjectReference
				ss.Name = secret
				build.Spec.Source.SourceSecret = &ss
			}
		}
	}
}

func getBuildEnv(build *buildapi.Build) *[]kapi.EnvVar {
	switch {
	case build.Spec.Strategy.DockerStrategy != nil:
		return &build.Spec.Strategy.DockerStrategy.Env
	case build.Spec.Strategy.SourceStrategy != nil:
		return &build.Spec.Strategy.SourceStrategy.Env
	case build.Spec.Strategy.CustomStrategy != nil:
		return &build.Spec.Strategy.CustomStrategy.Env
	}
	return nil
}

func addDefaultEnvVar(v kapi.EnvVar, envVars *[]kapi.EnvVar) {
	if envVars == nil {
		return
	}

	for i := range *envVars {
		if (*envVars)[i].Name == v.Name {
			return
		}
	}
	*envVars = append(*envVars, v)
}

func addDefaultLabel(defaultLabel buildapi.ImageLabel, buildLabels *[]buildapi.ImageLabel) {
	found := false
	for _, lbl := range *buildLabels {
		if lbl.Name == defaultLabel.Name {
			found = true
		}
	}
	if !found {
		*buildLabels = append(*buildLabels, defaultLabel)
	}
}

//setDefaultSourceSecret method check if sourcesecret is set via defaults options and return result or err in non found
func (a *buildDefaults) setDefaultSourceSecret(build *buildapi.Build) (string, error) {
	//check if project annotion is set for default SourceSecret
	//second option is global config.
	ns, err := a.cache.GetNamespace(build.Namespace)
	if err == nil {
		if contains(ns.Annotations, "openshift.io/sourceSecret") {
			annotation := ns.Annotations["openshift.io/sourceSecret"]
			if len(annotation) > 0 {
				//todo add checks if secret exist.
				glog.V(5).Infof("Setting SourceSecret from project annotation on %s/%s . Secret name:  %s", build.Namespace, build.Name, annotation)
				return annotation, nil
			}
		} else {
			if len(a.defaultsConfig.SourceSecret) != 0 {
				//todo add checks if secret exist.
				glog.V(5).Infof("Setting sourceSecret from defaultConfig on %s/%s . Secret name: %s", build.Namespace, build.Name, a.defaultsConfig.SourceSecret)
				return a.defaultsConfig.SourceSecret, nil
			}
		}
	}
	glog.V(5).Infof("No default sourceSecret for %s/%s in admission plugin", build.Namespace, build.Name)
	return "", fmt.Errorf("No default sourceSecret found. default behaviour for %s/%s", build.Namespace, build.Name)
}

func contains(s map[string]string, e string) bool {
	for k := range s {
		if k == e {
			return true
		}
	}
	return false
}
