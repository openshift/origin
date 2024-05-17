package features

import (
	"os"

	"k8s.io/apimachinery/pkg/util/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/klog/v2"
)

// This override should only be used with a support exception. It provides a way
// to set the NewVolumeManagerReconstruction feature gate in a way that still
// allows the cluster to be upgradable. This is only needed on 4.14, since the
// feature gate defaults to true on 4.15 and later. Owners: @jsafrane @dobsonj
func OpenShiftNewVolumeManagerReconstructionOverride() {
	overrideVariable := "OCP_4_14_SUPPORT_EXCEPTION_ENABLE_NEW_VOLUME_MANAGER_RECONSTRUCTION"
	_, enabled := os.LookupEnv(overrideVariable)
	if enabled {
		klog.Infof("Environment variable %s is set, setting feature gate %s to true", overrideVariable, string(NewVolumeManagerReconstruction))
		fg := map[string]bool{string(NewVolumeManagerReconstruction): true}
		runtime.Must(utilfeature.DefaultMutableFeatureGate.SetFromMap(fg))
	}
}

func init() {
	OpenShiftNewVolumeManagerReconstructionOverride()
}
