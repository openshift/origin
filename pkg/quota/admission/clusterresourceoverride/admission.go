package clusterresourceoverride

import (
	"fmt"
	"io"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/plugin/pkg/admission/limitranger"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
	"github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api/validation"
)

const (
	clusterResourceOverrideAnnotation = "quota.openshift.io/cluster-resource-override-enabled"
	cpuBaseScaleFactor                = 1000.0 / (1024.0 * 1024.0 * 1024.0) // 1000 milliCores per 1GiB
)

var (
	cpuFloor = resource.MustParse("1m")
	memFloor = resource.MustParse("1Mi")
)

func init() {
	admission.RegisterPlugin(api.PluginName, func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
		pluginConfig, err := ReadConfig(config)
		if err != nil {
			return nil, err
		}
		if pluginConfig == nil {
			glog.Infof("Admission plugin %q is not configured so it will be disabled.", api.PluginName)
			return nil, nil
		}
		return newClusterResourceOverride(client, pluginConfig)
	})
}

type internalConfig struct {
	limitCPUToMemoryRatio     float64
	cpuRequestToLimitRatio    float64
	memoryRequestToLimitRatio float64
}
type clusterResourceOverridePlugin struct {
	*admission.Handler
	config       *internalConfig
	ProjectCache *cache.ProjectCache
	LimitRanger  admission.Interface
}
type limitRangerActions struct{}

var _ = oadmission.WantsProjectCache(&clusterResourceOverridePlugin{})
var _ = oadmission.Validator(&clusterResourceOverridePlugin{})
var _ = limitranger.LimitRangerActions(&limitRangerActions{})

// newClusterResourceOverride returns an admission controller for containers that
// configurably overrides container resource request/limits
func newClusterResourceOverride(client clientset.Interface, config *api.ClusterResourceOverrideConfig) (admission.Interface, error) {
	glog.V(2).Infof("%s admission controller loaded with config: %v", api.PluginName, config)
	var internal *internalConfig
	if config != nil {
		internal = &internalConfig{
			limitCPUToMemoryRatio:     float64(config.LimitCPUToMemoryPercent) / 100,
			cpuRequestToLimitRatio:    float64(config.CPURequestToLimitPercent) / 100,
			memoryRequestToLimitRatio: float64(config.MemoryRequestToLimitPercent) / 100,
		}
	}

	limitRanger, err := limitranger.NewLimitRanger(client, nil)
	if err != nil {
		return nil, err
	}

	return &clusterResourceOverridePlugin{
		Handler:     admission.NewHandler(admission.Create),
		config:      internal,
		LimitRanger: limitRanger,
	}, nil
}

// these serve to satisfy the interface so that our kept LimitRanger limits nothing and only provides defaults.
func (d *limitRangerActions) SupportsAttributes(a admission.Attributes) bool {
	return true
}
func (d *limitRangerActions) SupportsLimit(limitRange *kapi.LimitRange) bool {
	return true
}
func (d *limitRangerActions) Limit(limitRange *kapi.LimitRange, resourceName string, obj runtime.Object) error {
	return nil
}

func (a *clusterResourceOverridePlugin) SetProjectCache(projectCache *cache.ProjectCache) {
	a.ProjectCache = projectCache
}

func ReadConfig(configFile io.Reader) (*api.ClusterResourceOverrideConfig, error) {
	obj, err := configlatest.ReadYAML(configFile)
	if err != nil {
		glog.V(5).Infof("%s error reading config: %v", api.PluginName, err)
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}
	config, ok := obj.(*api.ClusterResourceOverrideConfig)
	if !ok {
		return nil, fmt.Errorf("unexpected config object: %#v", obj)
	}
	glog.V(5).Infof("%s config is: %v", api.PluginName, config)
	if errs := validation.Validate(config); len(errs) > 0 {
		return nil, errs.ToAggregate()
	}

	return config, nil
}

func (a *clusterResourceOverridePlugin) Validate() error {
	if a.ProjectCache == nil {
		return fmt.Errorf("%s did not get a project cache", api.PluginName)
	}
	return nil
}

// TODO this will need to update when we have pod requests/limits
func (a *clusterResourceOverridePlugin) Admit(attr admission.Attributes) error {
	glog.V(6).Infof("%s admission controller is invoked", api.PluginName)
	if a.config == nil || attr.GetResource().GroupResource() != kapi.Resource("pods") || attr.GetSubresource() != "" {
		return nil // not applicable
	}
	pod, ok := attr.GetObject().(*kapi.Pod)
	if !ok {
		return admission.NewForbidden(attr, fmt.Errorf("unexpected object: %#v", attr.GetObject()))
	}
	glog.V(5).Infof("%s is looking at creating pod %s in project %s", api.PluginName, pod.Name, attr.GetNamespace())

	// allow annotations on project to override
	if ns, err := a.ProjectCache.GetNamespace(attr.GetNamespace()); err != nil {
		glog.Warningf("%s got an error retrieving namespace: %v", api.PluginName, err)
		return admission.NewForbidden(attr, err) // this should not happen though
	} else {
		projectEnabledPlugin, exists := ns.Annotations[clusterResourceOverrideAnnotation]
		if exists && projectEnabledPlugin != "true" {
			glog.V(5).Infof("%s is disabled for project %s", api.PluginName, attr.GetNamespace())
			return nil // disabled for this project, do nothing
		}
	}

	// Reuse LimitRanger logic to apply limit/req defaults from the project. Ignore validation
	// errors, assume that LimitRanger will run after this plugin to validate.
	glog.V(5).Infof("%s: initial pod limits are: %#v", api.PluginName, pod.Spec)
	if err := a.LimitRanger.Admit(attr); err != nil {
		glog.V(5).Infof("%s: error from LimitRanger: %#v", api.PluginName, err)
	}
	glog.V(5).Infof("%s: pod limits after LimitRanger: %#v", api.PluginName, pod.Spec)
	for i := range pod.Spec.InitContainers {
		updateContainerResources(a.config, &pod.Spec.InitContainers[i])
	}
	for i := range pod.Spec.Containers {
		updateContainerResources(a.config, &pod.Spec.Containers[i])
	}
	glog.V(5).Infof("%s: pod limits after overrides are: %#v", api.PluginName, pod.Spec)
	return nil
}

func updateContainerResources(config *internalConfig, container *kapi.Container) {
	resources := container.Resources
	memLimit, memFound := resources.Limits[kapi.ResourceMemory]
	if memFound && config.memoryRequestToLimitRatio != 0 {
		// memory is measured in whole bytes.
		// the plugin rounds down to the nearest MiB rather than bytes to improve ease of use for end-users.
		amount := memLimit.Value() * int64(config.memoryRequestToLimitRatio*100) / 100
		// TODO: move into resource.Quantity
		var mod int64
		switch memLimit.Format {
		case resource.BinarySI:
			mod = 1024 * 1024
		default:
			mod = 1000 * 1000
		}
		if rem := amount % mod; rem != 0 {
			amount = amount - rem
		}
		q := resource.NewQuantity(int64(amount), memLimit.Format)
		if memFloor.Cmp(*q) > 0 {
			q = memFloor.Copy()
		}
		resources.Requests[kapi.ResourceMemory] = *q
	}
	if memFound && config.limitCPUToMemoryRatio != 0 {
		amount := float64(memLimit.Value()) * config.limitCPUToMemoryRatio * cpuBaseScaleFactor
		q := resource.NewMilliQuantity(int64(amount), resource.DecimalSI)
		if cpuFloor.Cmp(*q) > 0 {
			q = cpuFloor.Copy()
		}
		resources.Limits[kapi.ResourceCPU] = *q
	}

	cpuLimit, cpuFound := resources.Limits[kapi.ResourceCPU]
	if cpuFound && config.cpuRequestToLimitRatio != 0 {
		amount := float64(cpuLimit.MilliValue()) * config.cpuRequestToLimitRatio
		q := resource.NewMilliQuantity(int64(amount), cpuLimit.Format)
		if cpuFloor.Cmp(*q) > 0 {
			q = cpuFloor.Copy()
		}
		resources.Requests[kapi.ResourceCPU] = *q
	}

}
