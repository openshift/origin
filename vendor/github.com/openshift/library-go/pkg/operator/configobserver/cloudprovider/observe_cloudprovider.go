package cloudprovider

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/operator/configobserver"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"

	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
)

const (
	cloudProviderConfFilePath = "/etc/kubernetes/static-pod-resources/configmaps/cloud-config/%s"
	configNamespace           = "openshift-config"
)

// InfrastructureLister lists infrastrucre information and allows resources to be synced
type InfrastructureLister interface {
	InfrastructureLister() configlistersv1.InfrastructureLister
	ResourceSyncer() resourcesynccontroller.ResourceSyncer
}

// NewCloudProviderObserver returns a new cloudprovider observer for syncing cloud provider specific
// information to controller-manager and api-server.
func NewCloudProviderObserver(targetNamespaceName string, cloudProviderNamePath, cloudProviderConfigPath []string) configobserver.ObserveConfigFunc {
	cloudObserver := &cloudProviderObserver{
		targetNamespaceName:     targetNamespaceName,
		cloudProviderNamePath:   cloudProviderNamePath,
		cloudProviderConfigPath: cloudProviderConfigPath,
	}
	return cloudObserver.ObserveCloudProviderNames
}

type cloudProviderObserver struct {
	targetNamespaceName     string
	cloudProviderNamePath   []string
	cloudProviderConfigPath []string
}

// ObserveCloudProviderNames observes the cloud provider from the global cluster infrastructure resource.
func (c *cloudProviderObserver) ObserveCloudProviderNames(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}) (ret map[string]interface{}, _ []error) {
	defer func() {
		ret = configobserver.Pruned(ret, c.cloudProviderConfigPath, c.cloudProviderNamePath)
	}()

	listers := genericListers.(InfrastructureLister)
	var errs []error
	observedConfig := map[string]interface{}{}

	infrastructure, err := listers.InfrastructureLister().Get("cluster")
	if errors.IsNotFound(err) {
		recorder.Warningf("ObserveCloudProviderNames", "Required infrastructures.%s/cluster not found", configv1.GroupName)
		return observedConfig, errs
	}
	if err != nil {
		return existingConfig, append(errs, err)
	}

	cloudProvider := getPlatformName(infrastructure.Status.Platform, recorder)
	if len(cloudProvider) > 0 {
		if err := unstructured.SetNestedStringSlice(observedConfig, []string{cloudProvider}, c.cloudProviderNamePath...); err != nil {
			errs = append(errs, err)
		}
	}

	sourceCloudConfigMap := infrastructure.Spec.CloudConfig.Name
	sourceCloudConfigNamespace := configNamespace
	sourceLocation := resourcesynccontroller.ResourceLocation{
		Namespace: sourceCloudConfigNamespace,
		Name:      sourceCloudConfigMap,
	}

	// we set cloudprovider configmap values only for some cloud providers.
	validCloudProviders := sets.NewString("azure", "gce", "openstack", "vsphere")
	if !validCloudProviders.Has(cloudProvider) {
		sourceCloudConfigMap = ""
	}

	if len(sourceCloudConfigMap) == 0 {
		sourceLocation = resourcesynccontroller.ResourceLocation{}
	}

	if err := listers.ResourceSyncer().SyncConfigMap(
		resourcesynccontroller.ResourceLocation{
			Namespace: c.targetNamespaceName,
			Name:      "cloud-config",
		},
		sourceLocation); err != nil {
		return existingConfig, append(errs, err)
	}

	if len(sourceCloudConfigMap) == 0 {
		return observedConfig, errs
	}

	// usually key will be simply config but we should refer it just in case
	staticCloudConfFile := fmt.Sprintf(cloudProviderConfFilePath, infrastructure.Spec.CloudConfig.Key)

	if err := unstructured.SetNestedStringSlice(observedConfig, []string{staticCloudConfFile}, c.cloudProviderConfigPath...); err != nil {
		recorder.Warningf("ObserveCloudProviderNames", "Failed setting cloud-config : %v", err)
		return existingConfig, append(errs, err)
	}

	existingCloudConfig, _, err := unstructured.NestedStringSlice(existingConfig, c.cloudProviderConfigPath...)
	if err != nil {
		errs = append(errs, err)
		// keep going on read error from existing config
	}

	if !equality.Semantic.DeepEqual(existingCloudConfig, []string{staticCloudConfFile}) {
		recorder.Eventf("ObserveCloudProviderNamesChanges", "CloudProvider config file changed to %s", staticCloudConfFile)
	}

	return observedConfig, errs
}

func getPlatformName(platformType configv1.PlatformType, recorder events.Recorder) string {
	cloudProvider := ""
	switch platformType {
	case "":
		recorder.Warningf("ObserveCloudProvidersFailed", "Required status.platform field is not set in infrastructures.%s/cluster", configv1.GroupName)
	case configv1.AWSPlatformType:
		cloudProvider = "aws"
	case configv1.AzurePlatformType:
		cloudProvider = "azure"
	case configv1.VSpherePlatformType:
		cloudProvider = "vsphere"
	case configv1.BareMetalPlatformType:
	case configv1.GCPPlatformType:
		cloudProvider = "gce"
	case configv1.LibvirtPlatformType:
	case configv1.OpenStackPlatformType:
		cloudProvider = "openstack"
	case configv1.NonePlatformType:
	default:
		// the new doc on the infrastructure fields requires that we treat an unrecognized thing the same bare metal.
		// TODO find a way to indicate to the user that we didn't honor their choice
		recorder.Warningf("ObserveCloudProvidersFailed", fmt.Sprintf("No recognized cloud provider platform found in infrastructures.%s/cluster.status.platform", configv1.GroupName))
	}
	return cloudProvider
}
