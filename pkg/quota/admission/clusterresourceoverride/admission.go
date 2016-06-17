package clusterresourceoverride

import (
	"fmt"
	"io"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api"
	"github.com/openshift/origin/pkg/quota/admission/clusterresourceoverride/api/validation"
	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/plugin/pkg/admission/limitranger"

	"github.com/golang/glog"
	"speter.net/go/exp/math/dec/inf"
)

const (
	clusterResourceOverrideAnnotation = "quota.openshift.io/cluster-resource-override-enabled"
	cpuBaseScaleFactor                = 1000.0 / (1024.0 * 1024.0 * 1024.0) // 1000 milliCores per 1GiB
)

var (
	zeroDec  = inf.NewDec(0, 0)
	miDec    = inf.NewDec(1024*1024, 0)
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
	limitCPUToMemoryRatio     *inf.Dec
	cpuRequestToLimitRatio    *inf.Dec
	memoryRequestToLimitRatio *inf.Dec
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
			limitCPUToMemoryRatio:     inf.NewDec(config.LimitCPUToMemoryPercent, 2),
			cpuRequestToLimitRatio:    inf.NewDec(config.CPURequestToLimitPercent, 2),
			memoryRequestToLimitRatio: inf.NewDec(config.MemoryRequestToLimitPercent, 2),
		}
	}

	limitRanger, err := limitranger.NewLimitRanger(client, &limitRangerActions{})
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
	glog.V(5).Infof("%s: initial pod limits are: %#v", api.PluginName, pod.Spec.Containers[0].Resources)
	if err := a.LimitRanger.Admit(attr); err != nil {
		glog.V(5).Infof("%s: error from LimitRanger: %#v", api.PluginName, err)
	}
	glog.V(5).Infof("%s: pod limits after LimitRanger are: %#v", api.PluginName, pod.Spec.Containers[0].Resources)
	for _, container := range pod.Spec.Containers {
		resources := container.Resources
		memLimit, memFound := resources.Limits[kapi.ResourceMemory]
		if memFound && a.config.memoryRequestToLimitRatio.Cmp(zeroDec) != 0 {
			// memory is measured in whole bytes.
			// the plugin rounds down to the nearest MiB rather than bytes to improve ease of use for end-users.
			amount := multiply(memLimit.Amount, a.config.memoryRequestToLimitRatio)
			roundDownToNearestMi := multiply(divide(amount, miDec, 0, inf.RoundDown), miDec)
			value := resource.Quantity{Amount: roundDownToNearestMi, Format: resource.BinarySI}
			if memFloor.Cmp(value) > 0 {
				value = *(memFloor.Copy())
			}
			resources.Requests[kapi.ResourceMemory] = value
		}
		if memFound && a.config.limitCPUToMemoryRatio.Cmp(zeroDec) != 0 {
			// float math is necessary here as there is no way to create an inf.Dec to represent cpuBaseScaleFactor < 0.001
			// cpu is measured in millicores, so we need to scale and round down the value to nearest millicore scale
			amount := multiply(inf.NewDec(int64(float64(memLimit.Value())*cpuBaseScaleFactor), 3), a.config.limitCPUToMemoryRatio)
			amount.Round(amount, 3, inf.RoundDown)
			value := resource.Quantity{Amount: amount, Format: resource.DecimalSI}
			if cpuFloor.Cmp(value) > 0 {
				value = *(cpuFloor.Copy())
			}
			resources.Limits[kapi.ResourceCPU] = value
		}
		cpuLimit, cpuFound := resources.Limits[kapi.ResourceCPU]
		if cpuFound && a.config.cpuRequestToLimitRatio.Cmp(zeroDec) != 0 {
			// cpu is measured in millicores, so we need to scale and round down the value to nearest millicore scale
			amount := multiply(cpuLimit.Amount, a.config.cpuRequestToLimitRatio)
			amount.Round(amount, 3, inf.RoundDown)
			value := resource.Quantity{Amount: amount, Format: resource.DecimalSI}
			if cpuFloor.Cmp(value) > 0 {
				value = *(cpuFloor.Copy())
			}
			resources.Requests[kapi.ResourceCPU] = value
		}
	}
	glog.V(5).Infof("%s: pod limits after overrides are: %#v", api.PluginName, pod.Spec.Containers[0].Resources)
	return nil
}

func multiply(x *inf.Dec, y *inf.Dec) *inf.Dec {
	return inf.NewDec(0, 0).Mul(x, y)
}

func divide(x *inf.Dec, y *inf.Dec, s inf.Scale, r inf.Rounder) *inf.Dec {
	return inf.NewDec(0, 0).QuoRound(x, y, s, r)
}
