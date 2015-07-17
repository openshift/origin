package keepalived

import (
	"fmt"
	"io"
	"os"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/ipfailover"
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
	if port < 1 {
		port = ipfailover.DefaultWatchPort
	}

	glog.V(4).Infof("KeepAlived IP Failover config: %q - WatchPort: %+v", p.Name, port)

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
	namespace, _, err := p.Factory.OpenShiftClientConfig.Namespace()
	if err != nil {
		return "", err
	}

	glog.V(4).Infof("KeepAlived IP Failover config: %q - namespace: %q", p.Name, namespace)

	return namespace, nil
}

// GetDeploymentConfig gets the deployment config associated with this IP Failover configurator plugin.
func (p *KeepalivedPlugin) GetDeploymentConfig() (*deployapi.DeploymentConfig, error) {
	osClient, _, err := p.Factory.Clients()
	if err != nil {
		return nil, fmt.Errorf("error getting client: %v", err)
	}

	namespace, err := p.GetNamespace()
	if err != nil {
		return nil, fmt.Errorf("error getting namespace: %v", err)
	}

	dc, err := osClient.DeploymentConfigs(namespace).Get(p.Name)
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

	dc, err := GenerateDeploymentConfig(p.Name, p.Options, selector)
	if err != nil {
		return nil, fmt.Errorf("error generating DeploymentConfig: %v", err)
	}

	configList := &kapi.List{Items: []runtime.Object{dc}}

	glog.V(4).Infof("KeepAlived IP Failover DeploymentConfig: %q - generated config: %+v", p.Name, configList)

	return configList, nil
}

// Create the config and services associated with this IP Failover configuration.
func (p *KeepalivedPlugin) Create(out io.Writer) error {
	namespace, err := p.GetNamespace()
	if err != nil {
		return fmt.Errorf("error getting Namespace: %v", err)
	}

	mapper, typer := p.Factory.Factory.Object()
	bulk := configcmd.Bulk{
		Mapper:            mapper,
		Typer:             typer,
		RESTClientFactory: p.Factory.Factory.RESTClient,

		After: configcmd.NewPrintNameOrErrorAfter(out, os.Stderr),
	}

	configList, err := p.Generate()
	if err != nil {
		return fmt.Errorf("error generating config: %v", err)
	}

	if errs := bulk.Create(configList, namespace); len(errs) != 0 {
		return fmt.Errorf("error creating config: %+v", errs)
	}

	glog.V(4).Infof("Created KeepAlived IP Failover DeploymentConfig: %q", p.Name)

	return nil
}
