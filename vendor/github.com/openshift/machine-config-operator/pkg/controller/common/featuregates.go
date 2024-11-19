package common

import (
	"context"
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/api/features"
	"github.com/openshift/library-go/pkg/operator/configobserver/featuregates"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func WaitForFeatureGatesReady(ctx context.Context, featureGateAccess featuregates.FeatureGateAccess) error {
	timeout := time.After(1 * time.Minute)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timed out waiting for FeatureGates to be ready")
		default:
			features, err := featureGateAccess.CurrentFeatureGates()
			if err == nil {
				enabled, disabled := GetEnabledDisabledFeatures(features)
				klog.Infof("FeatureGates initialized: enabled=%v, disabled=%v", enabled, disabled)
				return nil
			}
			klog.Infof("Waiting for FeatureGates to be ready...")
			time.Sleep(1 * time.Second)
		}
	}
}

// getEnabledDisabledFeatures extracts enabled and disabled features from the feature gate.
func GetEnabledDisabledFeatures(features featuregates.FeatureGate) ([]string, []string) {
	var enabled []string
	var disabled []string

	for _, feature := range features.KnownFeatures() {
		if features.Enabled(feature) {
			enabled = append(enabled, string(feature))
		} else {
			disabled = append(disabled, string(feature))
		}
	}

	return enabled, disabled
}

// IsBootImageControllerRequired checks that the currently enabled feature gates and
// the platform of the cluster requires a boot image controller. If any errors are
// encountered, it will log them and return false.
// Current valid feature gate and platform combinations:
// GCP -> FeatureGateManagedBootImages
// AWS -> FeatureGateManagedBootImagesAWS
func IsBootImageControllerRequired(ctx *ControllerContext) bool {
	configClient := ctx.ClientBuilder.ConfigClientOrDie("ensure-boot-image-infra-client")
	infra, err := configClient.ConfigV1().Infrastructures().Get(context.TODO(), "cluster", metav1.GetOptions{})
	if err != nil {
		klog.Errorf("unable to get infrastructures for boot image controller startup: %v", err)
		return false
	}
	if infra.Status.PlatformStatus == nil {
		klog.Errorf("unable to get infra.Status.PlatformStatus for boot image controller startup: %v", err)
		return false
	}
	fg, err := ctx.FeatureGateAccess.CurrentFeatureGates()
	if err != nil {
		klog.Errorf("unable to get features for boot image controller startup: %v", err)
		return false
	}
	switch infra.Status.PlatformStatus.Type {
	case configv1.AWSPlatformType:
		return fg.Enabled(features.FeatureGateManagedBootImagesAWS)
	case configv1.GCPPlatformType:
		return fg.Enabled(features.FeatureGateManagedBootImages)
	}
	return false
}
