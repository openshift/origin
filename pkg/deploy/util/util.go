package util

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/util/namer"
)

// LatestDeploymentNameForConfig returns a stable identifier for config based on its version.
func LatestDeploymentNameForConfig(config *deployapi.DeploymentConfig) string {
	return fmt.Sprintf("%s-%d", config.Name, config.LatestVersion)
}

// LatestDeploymentInfo returns info about the latest deployment for a config,
// if it exists and its current status
func LatestDeploymentInfo(config *deployapi.DeploymentConfig, deployments *api.ReplicationControllerList) (bool, deployapi.DeploymentStatus) {
	if config.LatestVersion == 0 || len(deployments.Items) == 0 {
		return false, deployapi.DeploymentStatus("")
	}
	sort.Sort(ByLatestVersionDesc(deployments.Items))
	candidate := &deployments.Items[0]
	return DeploymentVersionFor(candidate) == config.LatestVersion, DeploymentStatusFor(candidate)
}

// DeployerPodSuffix is the suffix added to pods created from a deployment
const DeployerPodSuffix = "deploy"

// DeployerPodNameForDeployment returns the name of a pod for a given deployment
func DeployerPodNameForDeployment(deployment string) string {
	return namer.GetPodName(deployment, DeployerPodSuffix)
}

// LabelForDeployment builds a string identifier for a Deployment.
func LabelForDeployment(deployment *api.ReplicationController) string {
	return fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name)
}

// LabelForDeploymentConfig builds a string identifier for a DeploymentConfig.
func LabelForDeploymentConfig(config *deployapi.DeploymentConfig) string {
	return fmt.Sprintf("%s/%s:%d", config.Namespace, config.Name, config.LatestVersion)
}

// ConfigSelector returns a label Selector which can be used to find all
// deployments for a DeploymentConfig.
//
// TODO: Using the annotation constant for now since the value is correct
// but we could consider adding a new constant to the public types.
func ConfigSelector(name string) labels.Selector {
	return labels.Set{deployapi.DeploymentConfigAnnotation: name}.AsSelector()
}

// DeployerPodSelector returns a label Selector which can be used to find all
// deployer pods associated with a deployment with name.
func DeployerPodSelector(name string) labels.Selector {
	return labels.Set{deployapi.DeployerPodForDeploymentLabel: name}.AsSelector()
}

// AnyDeployerPodSelector returns a label Selector which can be used to find
// all deployer pods across all deployments, including hook and custom
// deployer pods.
func AnyDeployerPodSelector() labels.Selector {
	sel, _ := labels.Parse(deployapi.DeployerPodForDeploymentLabel)
	return sel
}

// HasChangeTrigger returns whether the provided deployment configuration has
// a config change trigger or not
func HasChangeTrigger(config *deployapi.DeploymentConfig) bool {
	for _, trigger := range config.Triggers {
		if trigger.Type == deployapi.DeploymentTriggerOnConfigChange {
			return true
		}
	}
	return false
}

// DecodeDeploymentConfig decodes a DeploymentConfig from controller using codec. An error is returned
// if the controller doesn't contain an encoded config.
func DecodeDeploymentConfig(controller *api.ReplicationController, codec runtime.Codec) (*deployapi.DeploymentConfig, error) {
	encodedConfig := []byte(EncodedDeploymentConfigFor(controller))
	if decoded, err := codec.Decode(encodedConfig); err == nil {
		if config, ok := decoded.(*deployapi.DeploymentConfig); ok {
			return config, nil
		} else {
			return nil, fmt.Errorf("decoded DeploymentConfig from controller is not a DeploymentConfig: %v", err)
		}
	} else {
		return nil, fmt.Errorf("failed to decode DeploymentConfig from controller: %v", err)
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
		return nil, fmt.Errorf("couldn't clone podSpec: %v", err)
	}

	controllerLabels := make(labels.Set)
	for k, v := range config.Labels {
		controllerLabels[k] = v
	}
	// Correlate the deployment with the config.
	// TODO: Using the annotation constant for now since the value is correct
	// but we could consider adding a new constant to the public types.
	controllerLabels[deployapi.DeploymentConfigAnnotation] = config.Name

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

func DeploymentConfigNameFor(obj runtime.Object) string {
	return annotationFor(obj, deployapi.DeploymentConfigAnnotation)
}

func DeploymentNameFor(obj runtime.Object) string {
	return annotationFor(obj, deployapi.DeploymentAnnotation)
}

func DeployerPodNameFor(obj runtime.Object) string {
	return annotationFor(obj, deployapi.DeploymentPodAnnotation)
}

func DeploymentStatusFor(obj runtime.Object) deployapi.DeploymentStatus {
	return deployapi.DeploymentStatus(annotationFor(obj, deployapi.DeploymentStatusAnnotation))
}

func DeploymentStatusReasonFor(obj runtime.Object) string {
	return annotationFor(obj, deployapi.DeploymentStatusReasonAnnotation)
}

func DeploymentDesiredReplicas(obj runtime.Object) (int, bool) {
	s := annotationFor(obj, deployapi.DesiredReplicasAnnotation)
	if len(s) == 0 {
		return 0, false
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return i, true
}

func EncodedDeploymentConfigFor(obj runtime.Object) string {
	return annotationFor(obj, deployapi.DeploymentEncodedConfigAnnotation)
}

func DeploymentVersionFor(obj runtime.Object) int {
	v, err := strconv.Atoi(annotationFor(obj, deployapi.DeploymentVersionAnnotation))
	if err != nil {
		return -1
	}
	return v
}

func IsDeploymentCancelled(deployment *api.ReplicationController) bool {
	value := annotationFor(deployment, deployapi.DeploymentCancelledAnnotation)
	return strings.EqualFold(value, deployapi.DeploymentCancelledAnnotationValue)
}

// IsTerminatedDeployment returns true if the passed deployment has terminated (either
// complete or failed).
func IsTerminatedDeployment(deployment *api.ReplicationController) bool {
	current := DeploymentStatusFor(deployment)
	return current == deployapi.DeploymentStatusComplete || current == deployapi.DeploymentStatusFailed
}

// annotationFor returns the annotation with key for obj.
func annotationFor(obj runtime.Object, key string) string {
	meta, err := api.ObjectMetaFor(obj)
	if err != nil {
		return ""
	}
	return meta.Annotations[key]
}

// ByLatestVersionAsc sorts deployments by LatestVersion ascending.
type ByLatestVersionAsc []api.ReplicationController

func (d ByLatestVersionAsc) Len() int      { return len(d) }
func (d ByLatestVersionAsc) Swap(i, j int) { d[i], d[j] = d[j], d[i] }
func (d ByLatestVersionAsc) Less(i, j int) bool {
	return DeploymentVersionFor(&d[i]) < DeploymentVersionFor(&d[j])
}

// ByLatestVersionDesc sorts deployments by LatestVersion descending.
type ByLatestVersionDesc []api.ReplicationController

func (d ByLatestVersionDesc) Len() int      { return len(d) }
func (d ByLatestVersionDesc) Swap(i, j int) { d[i], d[j] = d[j], d[i] }
func (d ByLatestVersionDesc) Less(i, j int) bool {
	return DeploymentVersionFor(&d[j]) < DeploymentVersionFor(&d[i])
}
