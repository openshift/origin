package podautoscaler

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
)

func overrideMappingsForOapiDeploymentConfig(mappings []*apimeta.RESTMapping, err error, targetGK schema.GroupKind) ([]*apimeta.RESTMapping, error) {
	if (targetGK == schema.GroupKind{Kind: "DeploymentConfig"}) {
		err = nil
		// NB: we don't convert to apps.openshift.io here since the patched scale client
		// will do it for us.
		mappings = []*apimeta.RESTMapping{
			{
				Resource:         "deploymentconfigs",
				GroupVersionKind: targetGK.WithVersion("v1"),
			},
		}
	}
	return mappings, err
}
