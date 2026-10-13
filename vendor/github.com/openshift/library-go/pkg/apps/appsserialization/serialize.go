package appsserialization

import (
	"fmt"

	appsv1 "github.com/openshift/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

func EncodeDeploymentConfig(config *appsv1.DeploymentConfig) ([]byte, error) {
	return runtime.Encode(annotationEncoder, config)
}
