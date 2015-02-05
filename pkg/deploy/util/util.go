package util

import (
	"encoding/json"
	"fmt"
	"hash/adler32"
	"strconv"

	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// LatestDeploymentNameForConfig returns a stable identifier for config based on its version.
func LatestDeploymentNameForConfig(config *deployapi.DeploymentConfig) string {
	return config.Name + "-" + strconv.Itoa(config.LatestVersion)
}

// HashPodSpecs hashes a PodSpec into a uint64.
// TODO: Resources are currently ignored due to the formats not surviving encoding/decoding
// in a consistent manner (e.g. 0 is represented sometimes as 0.000)
func HashPodSpec(t api.PodSpec) uint64 {
	// Ignore resources by making them uniformly empty
	for i := range t.Containers {
		t.Containers[i].Resources = api.ResourceRequirementSpec{}
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
			Name: LatestDeploymentNameForConfig(config),
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
