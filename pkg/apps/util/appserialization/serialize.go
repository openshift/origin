package appserialization

import (
	"fmt"
	"reflect"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/diff"

	appsv1 "github.com/openshift/api/apps/v1"
)

// DecodeDeploymentConfig decodes a DeploymentConfig from controller using annotation codec.
// An error is returned if the controller doesn't contain an encoded config or decoding fail.
func DecodeDeploymentConfig(controller metav1.ObjectMetaAccessor) (*appsv1.DeploymentConfig, error) {
	encodedConfig, exists := controller.GetObjectMeta().GetAnnotations()[appsv1.DeploymentEncodedConfigAnnotation]
	if !exists {
		return nil, fmt.Errorf("object %s does not have encoded deployment config annotation", controller.GetObjectMeta().GetName())
	}
	config, err := runtime.Decode(annotationDecoder, []byte(encodedConfig))
	if err != nil {
		return nil, err
	}
	externalConfig, ok := config.(*appsv1.DeploymentConfig)
	if !ok {
		return nil, fmt.Errorf("object %+v is not v1.DeploymentConfig", config)
	}
	return externalConfig, nil
}

// HasLatestPodTemplate checks for differences between current deployment config
// template and deployment config template encoded in the latest replication
// controller. If they are different it will return an string diff containing
// the change.
func HasLatestPodTemplate(currentConfig *appsv1.DeploymentConfig, rc *v1.ReplicationController) (bool, string, error) {
	latestConfig, err := DecodeDeploymentConfig(rc)
	if err != nil {
		return true, "", err
	}
	// The latestConfig represents an encoded DC in the latest deployment (RC).
	// TODO: This diverges from the upstream behavior where we compare deployment
	// template vs. replicaset template. Doing that will disallow any
	// modifications to the RC the deployment config controller create and manage
	// as a change to the RC will cause the DC to be reconciled and ultimately
	// trigger a new rollout because of skew between latest RC template and DC
	// template.
	if reflect.DeepEqual(currentConfig.Spec.Template, latestConfig.Spec.Template) {
		return true, "", nil
	}
	return false, diff.ObjectReflectDiff(currentConfig.Spec.Template, latestConfig.Spec.Template), nil
}

func EncodeDeploymentConfig(config *appsv1.DeploymentConfig) ([]byte, error) {
	return runtime.Encode(annotationEncoder, config)
}
