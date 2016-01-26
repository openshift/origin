package httpproxy

import (
	"fmt"
	"io"
	"io/ioutil"
	"reflect"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	buildapi "github.com/openshift/origin/pkg/build/api"
	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
)

func init() {
	admission.RegisterPlugin("BuildHTTPProxy", func(c kclient.Interface, config io.Reader) (admission.Interface, error) {
		proxyConfig, err := readConfig(config)
		if err != nil {
			return nil, err
		}

		return NewBuildHTTPProxy(proxyConfig), nil
	})
}

func readConfig(reader io.Reader) (*ProxyConfig, error) {
	if reader == nil || reflect.ValueOf(reader).IsNil() {
		return nil, nil
	}

	configBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	config := &ProxyConfig{}
	err = configlatest.ReadYAML(configBytes, config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

type buildHTTPProxy struct {
	*admission.Handler
	proxyConfig *ProxyConfig
}

// NewBuildHTTPProxy returns an admission control for builds that checks
// on policy based on the build strategy type
func NewBuildHTTPProxy(proxyConfig *ProxyConfig) admission.Interface {
	return &buildHTTPProxy{
		Handler:     admission.NewHandler(admission.Create, admission.Update),
		proxyConfig: proxyConfig,
	}
}

func (a *buildHTTPProxy) Admit(attributes admission.Attributes) error {
	if a.proxyConfig == nil {
		return nil
	}
	if attributes.GetResource() != "buildconfigs" {
		return nil
	}
	if len(attributes.GetSubresource()) != 0 {
		return nil
	}

	buildConfig, ok := attributes.GetObject().(*buildapi.BuildConfig)
	if !ok {
		return admission.NewForbidden(attributes, fmt.Errorf("unrecognized request object %#v", attributes.GetObject()))
	}

	if len(a.proxyConfig.HTTPProxy) != 0 {
		var envVars *[]kapi.EnvVar
		switch {
		case buildConfig.Spec.Strategy.DockerStrategy != nil:
			envVars = &buildConfig.Spec.Strategy.DockerStrategy.Env
		case buildConfig.Spec.Strategy.SourceStrategy != nil:
			envVars = &buildConfig.Spec.Strategy.SourceStrategy.Env
		case buildConfig.Spec.Strategy.CustomStrategy != nil:
			envVars = &buildConfig.Spec.Strategy.CustomStrategy.Env
		}

		found := false
		for i := range *envVars {
			if (*envVars)[i].Name == "HTTP_PROXY" {
				found = true
			}
		}
		if !found {
			*envVars = append(*envVars, kapi.EnvVar{Name: "HTTP_PROXY", Value: a.proxyConfig.HTTPProxy})
		}

		if buildConfig.Spec.Source.Git != nil {
			if buildConfig.Spec.Source.Git.HTTPProxy == nil {
				t := a.proxyConfig.HTTPProxy
				buildConfig.Spec.Source.Git.HTTPProxy = &t
			}
		}
	}

	if len(a.proxyConfig.HTTPSProxy) != 0 {
		var envVars *[]kapi.EnvVar
		switch {
		case buildConfig.Spec.Strategy.DockerStrategy != nil:
			envVars = &buildConfig.Spec.Strategy.DockerStrategy.Env
		case buildConfig.Spec.Strategy.SourceStrategy != nil:
			envVars = &buildConfig.Spec.Strategy.SourceStrategy.Env
		case buildConfig.Spec.Strategy.CustomStrategy != nil:
			envVars = &buildConfig.Spec.Strategy.CustomStrategy.Env
		}

		found := false
		for i := range *envVars {
			if (*envVars)[i].Name == "HTTPS_PROXY" {
				found = true
			}
		}
		if !found {
			*envVars = append(*envVars, kapi.EnvVar{Name: "HTTPS_PROXY", Value: a.proxyConfig.HTTPSProxy})
		}

		if buildConfig.Spec.Source.Git != nil {
			if buildConfig.Spec.Source.Git.HTTPSProxy == nil {
				t := a.proxyConfig.HTTPSProxy
				buildConfig.Spec.Source.Git.HTTPSProxy = &t
			}
		}
	}

	return nil
}
