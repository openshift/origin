package util

import (
	"encoding/json"
	"fmt"
	"hash/adler32"
	"strconv"
	"strings"

	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// LatestDeploymentIDForConfig returns a stable identifier for config based on its version.
func LatestDeploymentIDForConfig(config *deployapi.DeploymentConfig) string {
	return config.Name + "-" + strconv.Itoa(config.LatestVersion)
}

func ParamsForImageChangeTrigger(config *deployapi.DeploymentConfig, repoName string) *deployapi.DeploymentTriggerImageChangeParams {
	if config == nil || config.Triggers == nil {
		return nil
	}

	for _, trigger := range config.Triggers {
		if trigger.Type == deployapi.DeploymentTriggerOnImageChange && trigger.ImageChangeParams.RepositoryName == repoName {
			return trigger.ImageChangeParams
		}
	}

	return nil
}

// Set a-b
func Difference(a, b util.StringSet) util.StringSet {
	diff := util.StringSet{}

	if a == nil || b == nil {
		return diff
	}

	for _, s := range a.List() {
		if !b.Has(s) {
			diff.Insert(s)
		}
	}

	return diff
}

// Returns a map of referenced image name to image version
func ReferencedImages(deployment *deployapi.Deployment) map[string]string {
	result := make(map[string]string)

	if deployment == nil {
		return result
	}

	for _, container := range deployment.ControllerTemplate.Template.Spec.Containers {
		name, version := ParseContainerImage(container.Image)
		result[name] = version
	}

	return result
}

func ParseContainerImage(image string) (string, string) {
	tokens := strings.Split(image, ":")
	return tokens[0], tokens[1]
}

// HashPodSpecs hashes a PodSpec into a uint64.
// TODO: Resources are currently ignored due to the formats not surviving encoding/decoding
// in a consistent manner (e.g. 0 is represented sometimes as 0.000)
func HashPodSpec(t api.PodSpec) uint64 {

	for i := range t.Containers {
		t.Containers[i].CPU = resource.Quantity{}
		t.Containers[i].Memory = resource.Quantity{}
	}

	jsonString, err := json.Marshal(t)
	if err != nil {
		glog.Errorf("An error occurred marshalling pod state: %v", err)
		return 0
	}
	hash := adler32.New()
	fmt.Fprintf(hash, "%s", jsonString)
	return uint64(hash.Sum32())
}

// PodSpecsEqual returns true if the given PodSpecs are the same.
func PodSpecsEqual(a, b api.PodSpec) bool {
	return HashPodSpec(a) == HashPodSpec(b)
}

// DecodeDeploymentConfig decodes a DeploymentConfig from controller using codec. An error is returned
// if the controller doesn't contain an encoded config.
func DecodeDeploymentConfig(controller *api.ReplicationController, codec runtime.Codec) (*deployapi.DeploymentConfig, error) {
	encodedConfig := []byte(controller.Annotations[deployapi.DeploymentEncodedConfigAnnotation])
	if decoded, err := codec.Decode(encodedConfig); err == nil {
		if config, ok := decoded.(*deployapi.DeploymentConfig); ok {
			return config, nil
		} else {
			return nil, fmt.Errorf("Decoded deploymentConfig from controller is not a DeploymentConfig: %v", err)
		}
	} else {
		return nil, fmt.Errorf("Failed to decode DeploymentConfig from controller: %v", err)
	}
}

// EncodeDeploymentConfig encodes config as a string using codec.
func EncodeDeploymentConfig(config *deployapi.DeploymentConfig, codec runtime.Codec) (string, error) {
	if bytes, err := codec.Encode(config); err == nil {
		return string(bytes[:]), nil
	} else {
		return "", err
	}
}

// MakeDeployment creates a deployment represented as a ReplicationController and based on the given
// DeploymentConfig. The controller replica count will be zero.
func MakeDeployment(config *deployapi.DeploymentConfig, codec runtime.Codec) (*api.ReplicationController, error) {
	var err error
	var encodedConfig string

	if encodedConfig, err = EncodeDeploymentConfig(config, codec); err != nil {
		return nil, err
	}

	deployment := &api.ReplicationController{
		ObjectMeta: api.ObjectMeta{
			Name: LatestDeploymentIDForConfig(config),
			Annotations: map[string]string{
				deployapi.DeploymentConfigAnnotation:        config.Name,
				deployapi.DeploymentStatusAnnotation:        string(deployapi.DeploymentStatusNew),
				deployapi.DeploymentEncodedConfigAnnotation: encodedConfig,
				deployapi.DeploymentVersionAnnotation:       strconv.Itoa(config.LatestVersion),
			},
			Labels: config.Labels,
		},
		Spec: config.Template.ControllerTemplate,
	}

	// The deployment should be inactive initially
	deployment.Spec.Replicas = 0

	// Ensure that pods created by this deployment controller can be safely associated back
	// to the controller, and that multiple deployment controllers for the same config don't
	// manipulate each others' pods.
	deployment.Spec.Template.Labels[deployapi.DeploymentConfigLabel] = config.Name
	deployment.Spec.Template.Labels[deployapi.DeploymentLabel] = deployment.Name
	deployment.Spec.Selector[deployapi.DeploymentConfigLabel] = config.Name
	deployment.Spec.Selector[deployapi.DeploymentLabel] = deployment.Name

	return deployment, nil
}
