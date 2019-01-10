package network

import (
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/listers/core/v1"

	"github.com/openshift/library-go/pkg/operator/events"
)

const (
	clusterConfigNamespace = "kube-system"
	clusterConfigName      = "cluster-config-v1"
)

// GetClusterCIDRs reads the cluster CIDRs from the install-config ConfigMap in the cluster.
func GetClusterCIDRs(lister v1.ConfigMapLister, recorder events.Recorder) ([]string, error) {
	clusterConfig, err := lister.ConfigMaps(clusterConfigNamespace).Get(clusterConfigName)
	if errors.IsNotFound(err) {
		recorder.Warningf("ObserveClusterCIDRFailed", "Required %s/%s config map not found", clusterConfigNamespace, clusterConfigName)
		glog.Warning("configmap/cluster-config-v1.kube-system: not found")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	installConfigYaml, ok := clusterConfig.Data["install-config"]
	if !ok {
		glog.Warning("configmap/cluster-config-v1.kube-system: install-config not found")
		recorder.Warningf("ObserveClusterCIDRFailed", "ConfigMap %s/%s does not have required 'install-config'", clusterConfigNamespace, clusterConfigName)
		return nil, nil
	}
	installConfig := map[string]interface{}{}
	err = yaml.Unmarshal([]byte(installConfigYaml), &installConfig)
	if err != nil {
		recorder.Warningf("ObserveRestrictedCIDRFailed", "Unable to decode install config: %v'", err)
		return nil, fmt.Errorf("unable to parse install-config: %s", err)
	}

	var clusterCIDRs []string
	clusterNetworks, _, err := unstructured.NestedSlice(installConfig, "networking", "clusterNetworks")
	if err != nil {
		return nil, fmt.Errorf("unabled to parse install-config: %s", err)
	}
	for i, n := range clusterNetworks {
		obj, ok := n.(map[string]interface{})
		if !ok {
			recorder.Warningf("ObserveRestrictedCIDRFailed", "Required networking.clusterNetworks field is not set in install-config")
			return nil, fmt.Errorf("unabled to parse install-config: expected networking.clusterNetworks[%d] to be an object, got: %#v", i, n)
		}
		cidr, _, err := unstructured.NestedString(obj, "cidr")
		if err != nil {
			return nil, fmt.Errorf("unabled to parse install-config: %v", err)
		}
		clusterCIDRs = append(clusterCIDRs, cidr)
	}
	// fallback to podCIDR
	if clusterNetworks == nil {
		podCIDR, _, err := unstructured.NestedString(installConfig, "networking", "podCIDR")
		if err != nil {
			return nil, fmt.Errorf("unable to parse install-config: %v", err)
		}
		if len(podCIDR) == 0 {
			return nil, fmt.Errorf("configmap/cluster-config-v1.kube-system: install-config.networking.clusterNetworks and install-config.networking.podCIDR not found")
		}
		clusterCIDRs = append(clusterCIDRs, podCIDR)
	}

	return clusterCIDRs, nil
}

// GetServiceCIDR reads the service IP range from the install-config ConfigMap in the cluster.
func GetServiceCIDR(lister v1.ConfigMapLister, recorder events.Recorder) (string, error) {
	clusterConfig, err := lister.ConfigMaps(clusterConfigNamespace).Get(clusterConfigName)
	if errors.IsNotFound(err) {
		glog.Warning("configmap/cluster-config-v1.kube-system: not found")
		recorder.Warningf("ObserveServiceClusterIPRangesFailed", "Required %s/%s config map not found", clusterConfigNamespace, clusterConfigName)
		return "", nil
	}
	if err != nil {
		return "", err
	}

	installConfigYaml, ok := clusterConfig.Data["install-config"]
	if !ok {
		glog.Warning("configmap/cluster-config-v1.kube-system: install-config not found")
		recorder.Warningf("ObserveServiceClusterIPRangesFailed", "ConfigMap %s/%s does not have required 'install-config'", clusterConfigNamespace, clusterConfigName)
		return "", nil
	}
	installConfig := map[string]interface{}{}
	err = yaml.Unmarshal([]byte(installConfigYaml), &installConfig)
	if err != nil {
		return "", fmt.Errorf("unable to parse install-config: %v", err)
	}

	serviceCIDR, _, err := unstructured.NestedString(installConfig, "networking", "serviceCIDR")
	if err != nil {
		return "", fmt.Errorf("unable to parse install-config: %v", err)
	}
	if len(serviceCIDR) == 0 {
		recorder.Warningf("ObserveServiceClusterIPRangesFailed", "Required networking.serviceCIDR field is not set in install-config")
		return "", fmt.Errorf("configmap/cluster-config-v1.kube-system: install-config.networking.serviceCIDR not found")
	}

	return serviceCIDR, nil
}
