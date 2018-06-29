package registry_operator

import (
	cryptorand "crypto/rand"
	"encoding/base64"
	"fmt"
	"reflect"

	"gopkg.in/yaml.v2"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	registryconfigv1 "github.com/openshift/api/dockerregistry/v1"
	operatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcecread"
	registryv1alpha1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apis/dockerregistry/v1alpha1"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/apis/operators/v1alpha1helpers"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/dockerregistry-operator/v310_00"
)

// randomSecretSize is the number of random bytes to generate.
const randomSecretSize = 32

// most of the time the sync method will be good for a large span of minor versions
func sync_v310_00_to_00(
	c DockerRegistryOperator,
	operatorConfig *registryv1alpha1.OpenShiftDockerRegistryConfig,
	previousAvailability *operatorsv1alpha1.VersionAvailablity,
) (operatorsv1alpha1.VersionAvailablity, []error) {
	versionAvailability := operatorsv1alpha1.VersionAvailablity{
		Version: operatorConfig.Spec.Version,
	}

	errors := []error{}
	var err error

	requiredNamespace := resourceread.ReadNamespaceV1OrDie([]byte(v310_00.NamespaceYaml))
	_, _, err = resourceapply.ApplyNamespace(c.corev1Client, requiredNamespace)
	if err != nil {
		errors = append(errors, err)
	}

	requiredService := resourceread.ReadServiceV1OrDie([]byte(v310_00.ServiceYaml))
	_, _, err = resourceapply.ApplyService(c.corev1Client, requiredService)
	if err != nil {
		errors = append(errors, err)
	}

	requiredSA := resourceread.ReadServiceAccountV1OrDie([]byte(v310_00.ServiceAccountYaml))
	_, saModified, err := resourceapply.ApplyServiceAccount(c.corev1Client, requiredSA)
	if err != nil {
		errors = append(errors, err)
	}

	// TODO create a new secret whenever the data value changes
	_, secretModified, err := ensureSecret_v310_00_to_00(c, operatorConfig.Spec)
	if err != nil {
		errors = append(errors, err)
	}

	forceDeployment := operatorConfig.ObjectMeta.Generation != operatorConfig.Status.ObservedGeneration
	if saModified { // SA modification can cause new tokens
		forceDeployment = true
	}
	if secretModified {
		forceDeployment = true
	}

	// our secrets are in order, now it is time to create the Deployment
	// TODO check basic preconditions here
	actualDeployment, _, err := ensureDeployment_v310_00_to_00(c, operatorConfig, previousAvailability, forceDeployment)
	if err != nil {
		errors = append(errors, err)
	}
	if actualDeployment != nil {
		versionAvailability.UpdatedReplicas = actualDeployment.Status.UpdatedReplicas
		versionAvailability.ReadyReplicas = actualDeployment.Status.ReadyReplicas
		versionAvailability.Generations = []operatorsv1alpha1.GenerationHistory{
			{
				Group: "apps", Resource: "Deployment",
				Namespace: targetNamespaceName, Name: "docker-registry",
				LastGeneration: actualDeployment.ObjectMeta.Generation,
			},
		}
	}

	v1alpha1helpers.SetErrors(&versionAvailability, errors...)

	return versionAvailability, errors
}

func mergeGenericMaps(defaults reflect.Value, required reflect.Value) reflect.Value {
	result := make(map[interface{}]interface{})
	processedKeys := make(map[interface{}]struct{})

	for _, vrk := range required.MapKeys() {
		vrv := required.MapIndex(vrk)
		vdv := defaults.MapIndex(vrk)
		if vdv.IsValid() {
			result[vrk.Interface()] = mergeGenericValues(vdv, vrv)
		} else {
			result[vrk.Interface()] = vrv.Interface()
		}
		result[vrk] = vrv
		processedKeys[vrk.Interface()] = struct{}{}
	}

	for _, vdk := range defaults.MapKeys() {
		if _, ok := processedKeys[vdk.Interface()]; ok {
			continue
		}
		result[vdk.Interface()] = defaults.MapIndex(vdk).Interface()
	}

	return reflect.ValueOf(result)
}

func mergeGenericValues(defaults reflect.Value, required reflect.Value) reflect.Value {
	if required.Type().Kind() != defaults.Type().Kind() || required.Type().Kind() != reflect.Map {
		return required
	}

	return mergeGenericMaps(defaults, required)
}

func mergeGenericInterfaces(defaults interface{}, required interface{}) interface{} {
	return mergeGenericValues(reflect.ValueOf(defaults), reflect.ValueOf(required))
}

// TopLevelRegistryConfig can be directly serialized to a yaml string that can be passed as the configuration
// file to docker registry.
type TopLevelRegistryConfig struct {
	Log       registryconfigv1.LogConfiguration `json:"log,omitempty"`
	Openshift OpenShiftRegistryConfig           `json:"openshift,omitempty"`
}

// OpenShiftRegistryConfig holds the openshift configuration section of the config file.
type OpenShiftRegistryConfig struct {
	Pullthrough *registryconfigv1.PullthroughConfiguration `json:"pullthrough,omitempty"`
}

func encodeRegistryConfig(rawConfig string, spec registryv1alpha1.OpenShiftDockerRegistryConfigSpec) ([]byte, error) {
	var decoded interface{}
	if len(rawConfig) == 0 {
		decoded = reflect.ValueOf(map[string]interface{}{}).Interface()
	} else {
		err := yaml.Unmarshal([]byte(rawConfig), decoded)
		if err != nil {
			return nil, fmt.Errorf("failed to parse raw config: %v", err)
		}
	}

	defaults := TopLevelRegistryConfig{
		Log: spec.RegistryConfig.Log,
		Openshift: OpenShiftRegistryConfig{
			Pullthrough: spec.RegistryConfig.Pullthrough,
		},
	}
	// TODO either pass the struct directly to the mergeGenericInterfaces
	// or find out more efficient way to turn struct to a map
	defaultsEncoded, err := yaml.Marshal(defaults)
	if err != nil {
		return nil, err
	}
	var defaultsDecoded interface{}
	if err := yaml.Unmarshal(defaultsEncoded, defaultsDecoded); err != nil {
		return nil, err
	}

	merged := mergeGenericInterfaces(decoded, defaultsDecoded)
	return yaml.Marshal(merged)
}

func ensureSecret_v310_00_to_00(
	c DockerRegistryOperator,
	options registryv1alpha1.OpenShiftDockerRegistryConfigSpec,
) (*corev1.Secret, bool, error) {
	requiredConfig, err := ensureDockerRegistryConfig(v310_00.DockerRegistryConfig, options)
	if err != nil {
		return nil, false, err
	}

	newDockerRegistryConfig, err := runtime.Encode(registryCodecs.LegacyCodec(registryconfigv1.SchemeGroupVersion), requiredConfig)
	if err != nil {
		return nil, false, err
	}
	requiredSecret := resourceread.ReadSecretV1OrDie([]byte(v310_00.SecretYaml))
	requiredSecret.Data[v310_00.ConfigSecretKey] = []byte(newDockerRegistryConfig)

	return resourceapply.ApplySecret(c.corev1Client, requiredSecret)
}

func makeContainerEnvironment(options *registryv1alpha1.OpenShiftDockerRegistryConfig) []corev1.EnvVar {
	env := make([]corev1.EnvVar, 0, len(options.Spec.RegistryConfig.Envs))
	for k, v := range options.Spec.RegistryConfig.Envs {
		env = append(env, corev1.EnvVar{Name: k, Value: v})
	}

	// one time http secret generation
	// TODO move to utility library and share with the `oc adm registry` code
	if len(options.Status.HttpSecret) == 0 {
		secretBytes := make([]byte, randomSecretSize)
		if _, err := cryptorand.Read(secretBytes); err != nil {
			utilruntime.HandleError(fmt.Errorf("could not generate random bytes for HTTP secret: %v", err))
			return env
		}
		options.Status.HttpSecret = base64.StdEncoding.EncodeToString(secretBytes)
	}

	env = append(env, corev1.EnvVar{Name: "REGISTRY_HTTP_SECRET", Value: options.Status.HttpSecret})
	return env
}

func ensureDeployment_v310_00_to_00(
	c DockerRegistryOperator,
	options *registryv1alpha1.OpenShiftDockerRegistryConfig,
	previousAvailability *operatorsv1alpha1.VersionAvailablity,
	forceDeployment bool,
) (*appsv1.Deployment, bool, error) {
	required := resourceread.ReadDeploymentV1OrDie([]byte(v310_00.DeploymentYaml))
	required.Spec.Template.Spec.Containers[0].Image = options.Spec.ImagePullSpec
	required.Spec.Template.Spec.Containers[0].Env = makeContainerEnvironment(options)
	required.Spec.Replicas = &options.Spec.Replicas
	required.Spec.Template.Spec.NodeSelector = options.Spec.NodeSelector

	generation := int64(-1)
	if previousAvailability != nil {
		for _, curr := range previousAvailability.Generations {
			if curr.Name == "docker-registry" {
				generation = curr.LastGeneration
			}
		}
	}
	return resourceapply.ApplyDeployment(c.appsv1Client, required, generation, forceDeployment)
}
