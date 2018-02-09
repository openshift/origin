package scale

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	oappsv1 "github.com/openshift/api/apps/v1"
)

var dcGVR = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "deploymentconfigs",
}

func correctOapiDeploymentConfig(gvr schema.GroupVersionResource) schema.GroupVersionResource {
	// TODO(directxman12): this is a dirty, dirty hack because oapi just appears in discovery as "/v1", like
	// the kube core API.  We can remove it if/when we get rid of the legacy oapi group entirely.  It makes me
	// cry a bit inside, but such is life.
	if gvr == dcGVR {
		return oappsv1.SchemeGroupVersion.WithResource(gvr.Resource)
	}

	return gvr
}
