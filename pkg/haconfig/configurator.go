package haconfig

import (
	"io"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/golang/glog"
)

type Configurator struct {
	Name   string
	Plugin HAConfiguratorPlugin
	Writer io.Writer
}

func NewConfigurator(name string, plugin HAConfiguratorPlugin, out io.Writer) *Configurator {
	glog.V(4).Infof("Creating haconfig configurator: %s", name)
	return &Configurator{Name: name, Plugin: plugin, Writer: out}
}

func (c *Configurator) Generate() *kapi.List {
	glog.V(4).Infof("Generating haconfig configuration: %s", c.Name)
	return c.Plugin.Generate()
}

func (c *Configurator) Create() {
	glog.V(4).Infof("Creating haconfig configuration: %s", c.Name)
	c.Plugin.Create(c.Writer)
}

func (c *Configurator) Delete() {
	glog.V(4).Infof("Deleting haconfig configuration: %s", c.Name)
	c.Plugin.Delete()
}
