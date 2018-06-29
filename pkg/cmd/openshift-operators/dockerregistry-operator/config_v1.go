package registry_operator

import (
	"fmt"

	registryconfigv1 "github.com/openshift/api/dockerregistry/v1"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	registryv1alpha1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apis/dockerregistry/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

func ensureDockerRegistryConfig(
	defaultConfigBytes string,
	options registryv1alpha1.OpenShiftDockerRegistryConfigSpec,
) (*registryconfigv1.DockerRegistryConfiguration, error) {
	mergedConfig := &registryconfigv1.DockerRegistryConfiguration{}
	defaultConfig, err := readDockerRegistryConfiguration(defaultConfigBytes)
	if err != nil {
		return nil, err
	}
	ensureDockerRegistryConfiguration(resourcemerge.BoolPtr(false), mergedConfig, *defaultConfig)
	ensureDockerRegistryConfiguration(resourcemerge.BoolPtr(false), mergedConfig, options.RegistryConfig)

	return mergedConfig, nil
}

var (
	registryScheme = runtime.NewScheme()
	registryCodecs = serializer.NewCodecFactory(registryScheme)
)

func init() {
	registryconfigv1.AddToScheme(registryScheme)
}

func readDockerRegistryConfiguration(objBytes string) (*registryconfigv1.DockerRegistryConfiguration, error) {
	defaultConfigObj, err := runtime.Decode(registryCodecs.UniversalDecoder(registryconfigv1.SchemeGroupVersion), []byte(objBytes))
	if err != nil {
		return nil, err
	}
	ret, ok := defaultConfigObj.(*registryconfigv1.DockerRegistryConfiguration)
	if !ok {
		return nil, fmt.Errorf("expected *registryconfigv1.DockerRegistryConfiguration, got %T", defaultConfigObj)
	}

	return ret, nil
}

// TODO this entire chain of methods needs to be taught the difference between specified and unspecified
func ensureDockerRegistryConfiguration(
	modified *bool,
	existing *registryconfigv1.DockerRegistryConfiguration,
	required registryconfigv1.DockerRegistryConfiguration,
) {
	resourcemerge.MergeMap(modified, &existing.Envs, required.Envs)
	// TODO: parse the yamls and merge them rather than override
	if len(existing.RawConfig) != 0 {
		resourcemerge.SetStringIfSet(modified, &existing.RawConfig, required.RawConfig)
	}
	ensureLogConfiguration(modified, &existing.Log, required.Log)
	if required.Pullthrough != nil {
		if existing.Pullthrough == nil {
			existing.Pullthrough = &registryconfigv1.PullthroughConfiguration{}
		}
		ensurePullthroughConfiguration(modified, existing.Pullthrough, *required.Pullthrough)
	}
}

func ensureLogConfiguration(modified *bool, existing *registryconfigv1.LogConfiguration, required registryconfigv1.LogConfiguration) {
	// TODO here's a neat side-effect.  You need to have everything be nil-able to know the difference between missing and explicitly set to "".
	resourcemerge.SetStringIfSet(modified, &existing.Level, required.Level)
}

func ensurePullthroughConfiguration(
	modified *bool,
	existing *registryconfigv1.PullthroughConfiguration,
	required registryconfigv1.PullthroughConfiguration,
) {
	// TODO here's a neat side-effect.  You need to have everything be nil-able to know the difference between missing and explicitly set to false.
	resourcemerge.SetBool(modified, &existing.Mirror, required.Mirror)
}
