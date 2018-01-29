package keepalived

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	"github.com/openshift/origin/pkg/oc/experimental/ipfailover/ipfailover"
	"github.com/openshift/origin/pkg/oc/generate/app"
)

// KeepalivedPlugin is an IP Failover configurator plugin for keepalived sidecar.
type KeepalivedPlugin struct {
	Name    string
	Factory *clientcmd.Factory
	Options *ipfailover.IPFailoverConfigCmdOptions
}

// NewIPFailoverConfiguratorPlugin creates a new IPFailoverConfigurator (keepalived) plugin instance.
func NewIPFailoverConfiguratorPlugin(name string, f *clientcmd.Factory, options *ipfailover.IPFailoverConfigCmdOptions) (*KeepalivedPlugin, error) {
	glog.V(4).Infof("Creating new KeepAlived plugin: %q", name)

	p := &KeepalivedPlugin{
		Name:    name,
		Factory: f,
		Options: options,
	}

	return p, nil
}

// GetWatchPort gets the port to monitor for the IP Failover configuration.
func (p *KeepalivedPlugin) GetWatchPort() (int, error) {
	port := p.Options.WatchPort
	if port < 1 || port > 65535 {
		glog.V(4).Infof("Warning: KeepAlived IP Failover config: %q - WatchPort: %d invalid, will default to %d", p.Name, port, ipfailover.DefaultWatchPort)
		port = ipfailover.DefaultWatchPort
	}

	glog.V(4).Infof("KeepAlived IP Failover config: %q - WatchPort: %d", p.Name, port)

	return port, nil
}

// GetSelector gets the selector associated with this IP Failover configurator plugin.
func (p *KeepalivedPlugin) GetSelector() (map[string]string, error) {
	labels := make(map[string]string, 0)

	if p.Options.Selector == ipfailover.DefaultSelector {
		return map[string]string{ipfailover.DefaultName: p.Name}, nil
	}

	labels, remove, err := app.LabelsFromSpec(strings.Split(p.Options.Selector, ","))
	if err != nil {
		return labels, err
	}

	if len(remove) > 0 {
		return labels, fmt.Errorf("you may not pass negative labels in %q", p.Options.Selector)
	}

	glog.V(4).Infof("KeepAlived IP Failover config: %q - selector: %+v", p.Name, labels)

	return labels, nil
}

// GetNamespace gets the namespace associated with this IP Failover configurator plugin.
func (p *KeepalivedPlugin) GetNamespace() (string, error) {
	namespace, _, err := p.Factory.DefaultNamespace()
	if err != nil {
		return "", err
	}

	glog.V(4).Infof("KeepAlived IP Failover config: %q - namespace: %q", p.Name, namespace)

	return namespace, nil
}

// GetDeploymentConfig gets the deployment config associated with this IP Failover configurator plugin.

func (p *KeepalivedPlugin) GetDeploymentConfig() (*appsapi.DeploymentConfig, error) {
	appsClient, err := p.Factory.OpenshiftInternalAppsClient()
	if err != nil {
		return nil, fmt.Errorf("error getting client: %v", err)
	}

	namespace, err := p.GetNamespace()
	if err != nil {
		return nil, fmt.Errorf("error getting namespace: %v", err)
	}

	dc, err := appsClient.Apps().DeploymentConfigs(namespace).Get(p.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			glog.V(4).Infof("KeepAlived IP Failover DeploymentConfig: %s not found", p.Name)
			return nil, nil
		}
		return nil, fmt.Errorf("error getting KeepAlived IP Failover DeploymentConfig %q: %v", p.Name, err)
	}

	glog.V(4).Infof("KeepAlived IP Failover DeploymentConfig: %q = %+v", p.Name, dc)

	return dc, nil
}

// Generate the config and services for this IP Failover configuration plugin.
func (p *KeepalivedPlugin) Generate() (*kapi.List, error) {
	selector, err := p.GetSelector()
	if err != nil {
		return nil, fmt.Errorf("error getting selector: %v", err)
	}

	if len(p.Options.VirtualIPs) == 0 {
		return nil, fmt.Errorf("you must specify at least one virtual IP address (--virtual-ips=) for keepalived to expose")
	}

	dc, err := GenerateDeploymentConfig(p.Name, p.Options, selector)
	if err != nil {
		return nil, fmt.Errorf("error generating DeploymentConfig: %v", err)
	}

	configList := &kapi.List{Items: []runtime.Object{dc}}

	glog.V(4).Infof("KeepAlived IP Failover DeploymentConfig: %q - generated config: %+v", p.Name, configList)

	return configList, nil
}
