package util

import (
	"encoding/json"
	"fmt"
	"hash/adler32"
	"strconv"
	"strings"

	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

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

func HashPodSpec(t api.PodSpec) uint64 {
	jsonString, err := json.Marshal(t)
	if err != nil {
		glog.Errorf("An error occurred marshalling pod state: %v", err)
		return 0
	}
	hash := adler32.New()
	fmt.Fprintf(hash, "%s", jsonString)
	return uint64(hash.Sum32())
}

func PodSpecsEqual(a, b api.PodSpec) bool {
	return HashPodSpec(a) == HashPodSpec(b)
}

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

func EncodeDeploymentConfig(config *deployapi.DeploymentConfig, codec runtime.Codec) (string, error) {
	if bytes, err := codec.Encode(config); err == nil {
		return string(bytes[:]), nil
	} else {
		return "", err
	}
}
