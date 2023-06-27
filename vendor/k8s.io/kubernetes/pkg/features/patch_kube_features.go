package features

import (
	"os"

	"k8s.io/apimachinery/pkg/util/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/featuregate"
)

var (
	// owner: @fbertina
	// beta: v1.23
	//
	// Enables the vSphere CSI migration for the Attach/Detach controller (ADC) only.
	ADCCSIMigrationVSphere featuregate.Feature = "ADC_CSIMigrationVSphere"
)

var ocpDefaultKubernetesFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	ADCCSIMigrationVSphere: {Default: true, PreRelease: featuregate.GA},
}

func OpenShiftStartCSIMigrationVSphere() bool {
	_, migrationEnabled := os.LookupEnv("OPENSHIFT_DO_VSPHERE_MIGRATION")
	return migrationEnabled
}

func init() {
	runtime.Must(utilfeature.DefaultMutableFeatureGate.Add(ocpDefaultKubernetesFeatureGates))
}
