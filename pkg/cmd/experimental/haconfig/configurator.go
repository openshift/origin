package haconfig

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
)

type Configurator struct {
	Name    string
	Factory *clientcmd.Factory
	Options *HAConfigCmdOptions
	Writer  io.Writer
}

func getConfigurationName(args []string) string {
	name := DefaultName

	switch len(args) {
	case 0:
		// Do nothing - use default name.
	case 1:
		name = args[0]
	default:
		glog.Fatalf("Please pass zero or one arguments to provide a name for this configuration.")
	}

	return name
}

func NewConfigurator(f *clientcmd.Factory, options *HAConfigCmdOptions, args []string, out io.Writer) *Configurator {
	return &Configurator{
		Name:    getConfigurationName(args),
		Factory: f,
		Options: options,
		Writer:  out,
	}
}

func (c *Configurator) GetWatchPort() kapi.ContainerPort {
	ports, err := app.ContainerPortsFromString(c.Options.WatchPort)
	if err != nil {
		glog.Fatal(err)
	}

	if len(ports) < 1 {
		glog.Fatal("Invalid WatchPort specified")
	}

	return ports[0]
}

func (c *Configurator) GetSelector() map[string]string {
	if c.Options.Selector == DefaultSelector {
		return map[string]string{DefaultName: c.Name}
	}

	labels, remove, err := app.LabelsFromSpec(strings.Split(c.Options.Selector, ","))
	if err != nil {
		glog.Fatal(err)
	}

	if len(remove) > 0 {
		glog.Fatalf("You may not pass negative labels in %q", c.Options.Selector)
	}

	return labels
}

func (c *Configurator) GetNamespace() string {
	namespace, err := c.Factory.OpenShiftClientConfig.Namespace()
	if err != nil {
		glog.Fatalf("Error get OS client config: %v", err)
	}

	return namespace
}

func (c *Configurator) GetService() *kapi.Service {
	_, kClient, err := c.Factory.Clients()
	if err != nil {
		glog.Fatalf("Error getting client: %v", err)
	}

	namespace := c.GetNamespace()
	service, err := kClient.Services(namespace).Get(c.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		glog.Fatalf("Error getting HA config service %q: %v", c.Name, err)
	}

	return service
}

func (c *Configurator) Generate() *kapi.List {
	dc := GenerateDeploymentConfig(c.Name, c.Options, c.GetSelector())
	objects := []runtime.Object{dc}

	return &kapi.List{Items: app.AddServices(objects)}
}

func (c *Configurator) Create() {
	namespace := c.GetNamespace()

	bulk := configcmd.Bulk{
		Factory: c.Factory.Factory,
		After:   configcmd.NewPrintNameOrErrorAfter(c.Writer, os.Stderr),
	}

	if errs := bulk.Create(c.Generate(), namespace); len(errs) != 0 {
		glog.Fatalf("Error creating config: %+v", errs)
	}
}

func (c *Configurator) Delete() {
	namespace := c.GetNamespace()

	_, kClient, err := c.Factory.Clients()
	if err != nil {
		glog.Fatalf("Error getting client: %v", err)
	}

	serviceInterface := kClient.Services(namespace)

	err = serviceInterface.Delete(c.Name)
	if err != nil {
		glog.Fatalf("Error deleting service %q: %v", c.Name, err)
	}

	// TODO(ramr): remove deployment config as well.
	glog.Infof("Deleted configuration for %q", c.Name)
}
