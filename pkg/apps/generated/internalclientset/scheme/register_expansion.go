package scheme

import (
	extensionsv1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
)

func init() {
	// Needed for GetScale/UpdateScale
	extensionsv1beta1.AddToScheme(Scheme)
}
