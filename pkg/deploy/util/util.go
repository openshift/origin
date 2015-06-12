package util

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployv3 "github.com/openshift/origin/pkg/deploy/api/v1beta3"
	"github.com/openshift/origin/pkg/util/namer"
)

// These constants represent keys used for correlating objects related to deployments.
const (
	// DeploymentConfigAnnotation is an annotation name used to correlate a deployment with the
	// DeploymentConfig on which the deployment is based.
	v1beta1DeploymentConfigAnnotation = "deploymentConfig"
	// DeploymentAnnotation is an annotation on a deployer Pod. The annotation value is the name
	// of the deployment (a ReplicationController) on which the deployer Pod acts.
	v1beta1DeploymentAnnotation = "deployment"
	// DeploymentPodAnnotation is an annotation on a deployment (a ReplicationController). The
	// annotation value is the name of the deployer Pod which will act upon the ReplicationController
	// to implement the deployment behavior.
	v1beta1DeploymentPodAnnotation = "pod"
	// DeploymentStatusAnnotation is an annotation name used to retrieve the DeploymentStatus of
	// a deployment.
	v1beta1DeploymentStatusAnnotation = "deploymentStatus"
	// DeploymentEncodedConfigAnnotation is an annotation name used to retrieve specific encoded
	// DeploymentConfig on which a given deployment is based.
	v1beta1DeploymentEncodedConfigAnnotation = "encodedDeploymentConfig"
	// DeploymentVersionAnnotation is an annotation on a deployment (a ReplicationController). The
	// annotation value is the LatestVersion value of the DeploymentConfig which was the basis for
	// the deployment.
	v1beta1DeploymentVersionAnnotation = "deploymentVersion"
	// DeploymentLabel is the name of a label used to correlate a deployment with the Pod created
	// to execute the deployment logic.
	// TODO: This is a workaround for upstream's lack of annotation support on PodTemplate. Once
	// annotations are available on PodTemplate, audit this constant with the goal of removing it.
	v1beta1DeploymentLabel = "deployment"
	// DeploymentConfigLabel is the name of a label used to correlate a deployment with the
	// DeploymentConfigs on which the deployment is based.
	v1beta1DeploymentConfigLabel = "deploymentconfig"
	// DeploymentStatusReasonAnnotation represents the reason for deployment being in a given state
	// Used for specifying the reason for cancellation or failure of a deployment
	v1beta1DeploymentStatusReasonAnnotation = "openshift.io/deployment.status-reason"
	// DeploymentCancelledAnnotation indicates that the deployment has been cancelled
	// The annotation value does not matter and its mere presence indicates cancellation
	v1beta1DeploymentCancelledAnnotation = "openshift.io/deployment.cancelled"
)

// Maps the latest annotation keys to all known previous key names. Keys not represented here
// may still be looked up directly via mappedAnnotationFor
var annotationMap = map[string][]string{
	deployapi.DeploymentConfigAnnotation: {
		v1beta1DeploymentConfigAnnotation,
		deployv3.DeploymentConfigAnnotation,
	},
	deployapi.DeploymentAnnotation: {
		v1beta1DeploymentAnnotation,
		deployv3.DeploymentAnnotation,
	},
	deployapi.DeploymentPodAnnotation: {
		v1beta1DeploymentPodAnnotation,
		deployv3.DeploymentPodAnnotation,
	},
	deployapi.DeploymentStatusAnnotation: {
		v1beta1DeploymentStatusAnnotation,
		deployv3.DeploymentPhaseAnnotation,
	},
	deployapi.DeploymentEncodedConfigAnnotation: {
		v1beta1DeploymentEncodedConfigAnnotation,
		deployv3.DeploymentEncodedConfigAnnotation,
	},
	deployapi.DeploymentVersionAnnotation: {
		v1beta1DeploymentVersionAnnotation,
		deployv3.DeploymentVersionAnnotation,
	},
	deployapi.DeploymentCancelledAnnotation: {
		v1beta1DeploymentCancelledAnnotation,
		deployv3.DeploymentCancelledAnnotation,
	},
}

// LatestDeploymentNameForConfig returns a stable identifier for config based on its version.
func LatestDeploymentNameForConfig(config *deployapi.DeploymentConfig) string {
	return fmt.Sprintf("%s-%d", config.Name, config.LatestVersion)
}

// DeployerPodSuffix is the suffix added to pods created from a deployment
const DeployerPodSuffix = "deploy"

// DeployerPodNameForDeployment returns the name of a pod for a given deployment
func DeployerPodNameForDeployment(deployment *api.ReplicationController) string {
	return namer.GetPodName(deployment.Name, DeployerPodSuffix)
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
	return mappedAnnotationFor(obj, deployapi.DeploymentConfigAnnotation)
}

func DeploymentNameFor(obj runtime.Object) string {
	return mappedAnnotationFor(obj, deployapi.DeploymentAnnotation)
}

func DeployerPodNameFor(obj runtime.Object) string {
	return mappedAnnotationFor(obj, deployapi.DeploymentPodAnnotation)
}

func DeploymentStatusFor(obj runtime.Object) deployapi.DeploymentStatus {
	return deployapi.DeploymentStatus(mappedAnnotationFor(obj, deployapi.DeploymentStatusAnnotation))
}

func DeploymentStatusReasonFor(obj runtime.Object) string {
	return mappedAnnotationFor(obj, deployapi.DeploymentStatusReasonAnnotation)
}

func DeploymentDesiredReplicas(obj runtime.Object) (int, bool) {
	s := mappedAnnotationFor(obj, deployapi.DesiredReplicasAnnotation)
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
	return mappedAnnotationFor(obj, deployapi.DeploymentEncodedConfigAnnotation)
}

func DeploymentVersionFor(obj runtime.Object) int {
	v, err := strconv.Atoi(mappedAnnotationFor(obj, deployapi.DeploymentVersionAnnotation))
	if err != nil {
		return -1
	}
	return v
}

func IsDeploymentCancelled(deployment *api.ReplicationController) bool {
	value := mappedAnnotationFor(deployment, deployapi.DeploymentCancelledAnnotation)
	return strings.EqualFold(value, deployapi.DeploymentCancelledAnnotationValue)
}

// mappedAnnotationFor finds the given annotation in obj using the annotation
// map to search all known key variants.
func mappedAnnotationFor(obj runtime.Object, key string) string {
	meta, err := api.ObjectMetaFor(obj)
	if err != nil {
		return ""
	}
	for _, mappedKey := range annotationMap[key] {
		if val, ok := meta.Annotations[mappedKey]; ok {
			return val
		}
	}
	if val, ok := meta.Annotations[key]; ok {
		return val
	}
	return ""
}

// DeploymentsByLatestVersionAsc sorts deployments by LatestVersion ascending.
type DeploymentsByLatestVersionAsc []api.ReplicationController

func (d DeploymentsByLatestVersionAsc) Len() int {
	return len(d)
}
func (d DeploymentsByLatestVersionAsc) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}
func (d DeploymentsByLatestVersionAsc) Less(i, j int) bool {
	return DeploymentVersionFor(&d[i]) < DeploymentVersionFor(&d[j])
}

// DeploymentsByLatestVersionAsc sorts deployments by LatestVersion
// descending.
type DeploymentsByLatestVersionDesc []api.ReplicationController

func (d DeploymentsByLatestVersionDesc) Len() int {
	return len(d)
}
func (d DeploymentsByLatestVersionDesc) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}
func (d DeploymentsByLatestVersionDesc) Less(i, j int) bool {
	return DeploymentVersionFor(&d[j]) < DeploymentVersionFor(&d[i])
}
