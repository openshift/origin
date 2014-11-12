package util

import (
	"encoding/json"
	"fmt"
	"hash/adler32"
	"strconv"
	"strings"

	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
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

	for _, container := range deployment.ControllerTemplate.PodTemplate.DesiredState.Manifest.Containers {
		name, version := ParseContainerImage(container.Image)
		result[name] = version
	}

	return result
}

func ParseContainerImage(image string) (string, string) {
	tokens := strings.Split(image, ":")
	return tokens[0], tokens[1]
}

func HashPodTemplate(t api.PodState) uint64 {
	jsonString, err := json.Marshal(t)
	if err != nil {
		glog.Errorf("An error occurred marshalling pod state: %v", err)
		return 0
	}
	hash := adler32.New()
	fmt.Fprintf(hash, "%s", jsonString)
	return uint64(hash.Sum32())
}

func PodTemplatesEqual(a, b api.PodTemplate) bool {
	return HashPodTemplate(a.DesiredState) == HashPodTemplate(b.DesiredState)
}
