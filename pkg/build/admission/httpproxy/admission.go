package httpproxy

import (
	"fmt"
	"io"
	"io/ioutil"
	"reflect"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
)

func init() {
	admission.RegisterPlugin("BuildDefaulter", func(c kclient.Interface, config io.Reader) (admission.Interface, error) {
		defaultConfig, err := readConfig(config)
		if err != nil {
			return nil, err
		}

		return NewBuildDefaulter(defaultConfig), nil
	})
}

func readConfig(reader io.Reader) (*DefaultConfig, error) {
	if reader == nil || reflect.ValueOf(reader).IsNil() {
		return nil, nil
	}

	configBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	config := &DefaultConfig{}
	err = configlatest.ReadYAML(configBytes, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

type buildDefaulter struct {
	*admission.Handler
	defaultConfig *DefaultConfig
}

// NewBuildDefaulter returns an admission control for builds that will
// set default values on a build pod from the global config.
func NewBuildDefaulter(defaultConfig *DefaultConfig) admission.Interface {
	return &buildDefaulter{
		Handler:       admission.NewHandler(admission.Create, admission.Update),
		defaultConfig: defaultConfig,
	}
}

func (a *buildDefaulter) Admit(attributes admission.Attributes) error {
	if a.defaultConfig == nil {
		return nil
	}

	if attributes.GetResource().Resource != string(kapi.ResourcePods) {
		return nil
	}
	if len(attributes.GetSubresource()) != 0 {
		return nil
	}

	pod, ok := attributes.GetObject().(*kapi.Pod)

	if !ok {
		return admission.NewForbidden(attributes, fmt.Errorf("unrecognized request object %#v", attributes.GetObject()))
	}

	if len(pod.Annotations[buildapi.BuildAnnotation]) != 0 {
		found := false
		for i, env := range pod.Spec.Containers[0].Env {
			if env.Name == "BUILD" {
				found = true
				build := &buildapi.Build{}
				if err := latest.Codec.DecodeInto([]byte(env.Value), build); err != nil {
					return fmt.Errorf("failed to decode build: %v", err)
				}

				// default the http proxy for git cloning
				if len(a.defaultConfig.HTTPProxy) != 0 && build.Spec.Strategy.SourceStrategy != nil && build.Spec.Source.Git != nil && len(*build.Spec.Source.Git.HTTPProxy) == 0 {
					build.Spec.Source.Git.HTTPProxy = &a.defaultConfig.HTTPProxy
				}

				// default the https proxy for git cloning
				if len(a.defaultConfig.HTTPSProxy) != 0 && build.Spec.Strategy.SourceStrategy != nil && build.Spec.Source.Git != nil && len(*build.Spec.Source.Git.HTTPSProxy) == 0 {
					build.Spec.Source.Git.HTTPSProxy = &a.defaultConfig.HTTPSProxy
				}

				var envVars *[]kapi.EnvVar
				switch {
				case build.Spec.Strategy.DockerStrategy != nil:
					envVars = &build.Spec.Strategy.DockerStrategy.Env
				case build.Spec.Strategy.SourceStrategy != nil:
					envVars = &build.Spec.Strategy.SourceStrategy.Env
				case build.Spec.Strategy.CustomStrategy != nil:
					envVars = &build.Spec.Strategy.CustomStrategy.Env
				}

				if envVars != nil {
					for _, defaultEnv := range a.defaultConfig.Env {
						exists := false
						for _, buildEnv := range *envVars {
							if buildEnv.Name == defaultEnv.Name {
								exists = true
							}
						}
						if !exists {
							*envVars = append(*envVars, defaultEnv)
						}
					}
				}

				data, err := latest.Codec.Encode(build)
				if err != nil {
					return fmt.Errorf("failed to encode build %s/%s: %v", build.Namespace, build.Name, err)
				}
				pod.Spec.Containers[0].Env[i].Value = string(data)

				break
			}
		}
		if !found {
			glog.Warningf("Unable to find and update build definition on pod %#v", *pod)
		}
	}

	return nil
}
