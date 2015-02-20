package util

import (
	"encoding/json"
	"fmt"
	"hash/adler32"
	"strconv"

	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// LatestDeploymentNameForConfig returns a stable identifier for config based on its version.
func LatestDeploymentNameForConfig(config *deployapi.DeploymentConfig) string {
	return config.Name + "-" + strconv.Itoa(config.LatestVersion)
}

func DeployerPodNameForDeployment(deployment *api.ReplicationController) string {
	return fmt.Sprintf("deploy-%s", deployment.Name)
}

// HashPodSpecs hashes a PodSpec into a uint64.
// TODO: Resources are currently ignored due to the formats not surviving encoding/decoding
// in a consistent manner (e.g. 0 is represented sometimes as 0.000)
func HashPodSpec(t api.PodSpec) uint64 {
	// Ignore resources by making them uniformly empty
	for i := range t.Containers {
		t.Containers[i].Resources = api.ResourceRequirements{}
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

	deploymentName := LatestDeploymentNameForConfig(config)

	podSpec := api.PodSpec{}
	if err := api.Scheme.Convert(&config.Template.ControllerTemplate.Template.Spec, &podSpec); err != nil {
		return nil, fmt.Errorf("Couldn't clone podTemplateSpec: %v", err)
	}

	controllerLabels := make(labels.Set)
	for k, v := range config.Labels {
		controllerLabels[k] = v
	}

	// Ensure that pods created by this deployment controller can be safely associated back
	// to the controller, and that multiple deployment controllers for the same config don't
	// manipulate each others' pods.
	selector := map[string]string{}
	for k, v := range config.Template.ControllerTemplate.Selector {
		selector[k] = v
	}
	selector[deployapi.DeploymentConfigLabel] = config.Name
	selector[deployapi.DeploymentLabel] = deploymentName

	podLabels := make(labels.Set)
	for k, v := range config.Template.ControllerTemplate.Template.Labels {
		podLabels[k] = v
	}
	podLabels[deployapi.DeploymentConfigLabel] = config.Name
	podLabels[deployapi.DeploymentLabel] = deploymentName

	podAnnotations := make(labels.Set)
	for k, v := range config.Template.ControllerTemplate.Template.Annotations {
		podAnnotations[k] = v
	}
	podAnnotations[deployapi.DeploymentAnnotation] = deploymentName
	podAnnotations[deployapi.DeploymentConfigAnnotation] = config.Name
	podAnnotations[deployapi.DeploymentVersionAnnotation] = strconv.Itoa(config.LatestVersion)

	deployment := &api.ReplicationController{
		ObjectMeta: api.ObjectMeta{
			Name: deploymentName,
			Annotations: map[string]string{
				deployapi.DeploymentConfigAnnotation:        config.Name,
				deployapi.DeploymentStatusAnnotation:        string(deployapi.DeploymentStatusNew),
				deployapi.DeploymentEncodedConfigAnnotation: encodedConfig,
				deployapi.DeploymentVersionAnnotation:       strconv.Itoa(config.LatestVersion),
			},
			Labels: controllerLabels,
		},
		Spec: api.ReplicationControllerSpec{
			// The deployment should be inactive initially
			Replicas: 0,
			Selector: selector,
			Template: &api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels:      podLabels,
					Annotations: podAnnotations,
				},
				Spec: podSpec,
			},
		},
	}

	return deployment, nil
}

// ListWatcherImpl is a pluggable ListWatcher.
// TODO: This has been incorporated upstream; replace during a future rebase.
type ListWatcherImpl struct {
	ListFunc  func() (runtime.Object, error)
	WatchFunc func(resourceVersion string) (watch.Interface, error)
}

func (lw *ListWatcherImpl) List() (runtime.Object, error) {
	return lw.ListFunc()
}

func (lw *ListWatcherImpl) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.WatchFunc(resourceVersion)
}
