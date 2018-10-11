package configdefault

import (
	"time"

	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/library-go/pkg/config/configdefaults"
	leaderelectionconverter "github.com/openshift/library-go/pkg/config/leaderelection"
)

func SetRecommendedOpenShiftControllerConfigDefaults(config *openshiftcontrolplanev1.OpenShiftControllerManagerConfig) {
	configdefaults.SetRecommendedHTTPServingInfoDefaults(config.ServingInfo)
	configdefaults.SetRecommendedKubeClientConfigDefaults(&config.KubeClientConfig)
	config.LeaderElection = leaderelectionconverter.LeaderElectionDefaulting(config.LeaderElection, "kube-system", "openshift-master-controllers")

	configdefaults.DefaultStringSlice(&config.Controllers, []string{"*"})

	configdefaults.DefaultString(&config.Network.ServiceNetworkCIDR, "10.0.0.0/24")

	if config.ImageImport.MaxScheduledImageImportsPerMinute == 0 {
		config.ImageImport.MaxScheduledImageImportsPerMinute = 60
	}
	if config.ImageImport.ScheduledImageImportMinimumIntervalSeconds == 0 {
		config.ImageImport.ScheduledImageImportMinimumIntervalSeconds = 15 * 60
	}

	configdefaults.DefaultString(&config.SecurityAllocator.UIDAllocatorRange, "1000000000-1999999999/10000")
	configdefaults.DefaultString(&config.SecurityAllocator.MCSAllocatorRange, "s0:/2")
	if config.SecurityAllocator.MCSLabelsPerProject == 0 {
		config.SecurityAllocator.MCSLabelsPerProject = 5
	}

	if config.ResourceQuota.MinResyncPeriod.Duration == 0 {
		config.ResourceQuota.MinResyncPeriod.Duration = 5 * time.Minute
	}
	if config.ResourceQuota.SyncPeriod.Duration == 0 {
		config.ResourceQuota.SyncPeriod.Duration = 12 * time.Hour
	}
	if config.ResourceQuota.ConcurrentSyncs == 0 {
		config.ResourceQuota.ConcurrentSyncs = 5
	}

	if config.ImageImport.MaxScheduledImageImportsPerMinute == 0 {
		config.ImageImport.MaxScheduledImageImportsPerMinute = 60
	}
	if config.ImageImport.ScheduledImageImportMinimumIntervalSeconds == 0 {
		config.ImageImport.ScheduledImageImportMinimumIntervalSeconds = 15 * 60 // 15 minutes
	}
}
