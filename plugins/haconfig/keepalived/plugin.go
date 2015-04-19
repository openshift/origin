package keepalived

import (
	"io"
	"os"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	"github.com/openshift/origin/pkg/generate/app"
	"github.com/openshift/origin/pkg/haconfig"
)

//  HA configurator plugin for keepalived sidecar.
type KeepAlivedPlugin struct {
	Name    string
	Factory *clientcmd.Factory
	Options *haconfig.HAConfigCmdOptions
}

//  Create a new HAConfigurator (keepalived) plugin instance.
func NewHAConfiguratorPlugin(name string, f *clientcmd.Factory, options *haconfig.HAConfigCmdOptions) (*KeepAlivedPlugin, error) {
	glog.V(4).Infof("Creating new KeepAlived plugin: %q", name)

	p := &KeepAlivedPlugin{
		Name:    name,
		Factory: f,
		Options: options,
	}

	return p, nil
}

//  Get the port to monitor for the HA configuration.
func (p *KeepAlivedPlugin) GetWatchPort() int {
	port := p.Options.WatchPort
	if port < 1 {
		port = haconfig.DefaultWatchPort
	}

	glog.V(4).Infof("KeepAlived HA config: %q - WatchPort: %+v", p.Name, port)

	return port
}

//  Get the selector associated with this HA configurator plugin.
func (p *KeepAlivedPlugin) GetSelector() map[string]string {
	if p.Options.Selector == haconfig.DefaultSelector {
		return map[string]string{haconfig.DefaultName: p.Name}
	}

	labels, remove, err := app.LabelsFromSpec(strings.Split(p.Options.Selector, ","))
	if err != nil {
		glog.Fatal(err)
	}

	if len(remove) > 0 {
		glog.Fatalf("You may not pass negative labels in %q", p.Options.Selector)
	}

	glog.V(4).Infof("KeepAlived HA config: %q - selector: %+v", p.Name, labels)

	return labels
}

//  Get the namespace associated with this HA configurator plugin.
func (p *KeepAlivedPlugin) GetNamespace() string {
	namespace, err := p.Factory.OpenShiftClientConfig.Namespace()
	if err != nil {
		glog.Fatalf("Error get OS client config: %v", err)
	}

	glog.V(4).Infof("KeepAlived HA config: %q - namespace: %q", p.Name, namespace)

	return namespace
}

//  Get the service associated with this HA configurator plugin.
func (p *KeepAlivedPlugin) GetService() *kapi.Service {
	_, kClient, err := p.Factory.Clients()
	if err != nil {
		glog.Fatalf("Error getting client: %v", err)
	}

	namespace := p.GetNamespace()
	service, err := kClient.Services(namespace).Get(p.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			glog.V(4).Infof("KeepAlived HA config: %s - no service found", p.Name)
			return nil
		}
		glog.Fatalf("Error getting KeepAlived HA config service %q: %v", p.Name, err)
	}

	glog.V(4).Infof("KeepAlived HA config: %q service: %+v", p.Name, service)

	return service
}

//  Generate the config and services for this HA configuration plugin.
func (p *KeepAlivedPlugin) Generate() *kapi.List {
	dc := GenerateDeploymentConfig(p.Name, p.Options, p.GetSelector())
	objects := []runtime.Object{dc}

	services := &kapi.List{Items: app.AddServices(objects)}
	glog.V(4).Infof("KeepAlived HA config: %q - generated services: %+v", p.Name, services)

	return services
}

//  Create the config and services associated with this HA configuration.
func (p *KeepAlivedPlugin) Create(out io.Writer) {
	namespace := p.GetNamespace()

	bulk := configcmd.Bulk{
		Factory: p.Factory.Factory,
		After:   configcmd.NewPrintNameOrErrorAfter(out, os.Stderr),
	}

	if errs := bulk.Create(p.Generate(), namespace); len(errs) != 0 {
		glog.Fatalf("Error creating config: %+v", errs)
	}

	glog.V(4).Infof("Created KeepAlived HA config: %q", p.Name)
}

//  Delete the config and services associated with this HA configuration.
func (p *KeepAlivedPlugin) Delete() {
	namespace := p.GetNamespace()

	_, kClient, err := p.Factory.Clients()
	if err != nil {
		glog.Fatalf("Error getting client: %v", err)
	}

	serviceInterface := kClient.Services(namespace)

	err = serviceInterface.Delete(p.Name)
	if err != nil {
		glog.Fatalf("Error deleting service %q: %v", p.Name, err)
	}

	// TODO(ramr): remove deployment config as well.
	glog.Infof("Deleted KeepAlived HA config: %q", p.Name)
}
