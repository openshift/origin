package scheme

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	servicecertsignerv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
)

var ConfigScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(operatorv1alpha1.Install(ConfigScheme))
	utilruntime.Must(servicecertsignerv1alpha1.Install(ConfigScheme))
}
