package ipfailover

import (
	"io"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/golang/glog"
)

type Configurator struct {
	Name   string
	Plugin IPFailoverConfiguratorPlugin
	Writer io.Writer
}

func NewConfigurator(name string, plugin IPFailoverConfiguratorPlugin, out io.Writer) *Configurator {
	glog.V(4).Infof("Creating IP failover configurator: %s", name)
	return &Configurator{Name: name, Plugin: plugin, Writer: out}
}

func (c *Configurator) Generate() (*kapi.List, error) {
	glog.V(4).Infof("Generating IP failover configuration: %s", c.Name)
	return c.Plugin.Generate()
}

func (c *Configurator) Create() error {
	glog.V(4).Infof("Creating IP failover configuration: %s", c.Name)
	return c.Plugin.Create(c.Writer)
}
