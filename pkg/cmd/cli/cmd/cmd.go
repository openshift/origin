package cmd

import (
	"fmt"
	"io"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kmeta "github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/meta"
	"github.com/openshift/origin/pkg/client"
	"github.com/spf13/cobra"
)

// OriginFactory provides a Factory which handles both Kubernetes and Origin
// objects
type OriginFactory struct {
	KubeClient      func(*cobra.Command, *kmeta.RESTMapping) (kubectl.RESTClient, error)
	KubeDescriber   func(cmd *cobra.Command, mapping *kmeta.RESTMapping) (kubectl.Describer, error)
	OriginDescriber func(cmd *cobra.Command, mapping *kmeta.RESTMapping) (kubectl.Describer, error)
	*kubecmd.Factory
}

// MultiRESTClient provides a REST client suitable for managing the given
// Resource Mappings. It determines the right REST client based on the
// MultiRESTMapper.
func (f *OriginFactory) MultiRESTClient(c *cobra.Command, m *kmeta.RESTMapping) (kubectl.RESTClient, error) {
	mapper, ok := f.Factory.Mapper.(meta.MultiRESTMapper)
	if !ok {
		return nil, fmt.Errorf("the factory mapper must be MultiRESTMapper")
	}
	switch mapper.APINameForResource(m.APIVersion, m.Kind) {
	case meta.OriginAPI:
		glog.V(3).Infof("Resource identified as '%s', serving using origin client", m.Kind)
		return f.OriginClient(c, m)
	case meta.KubernetesAPI:
		glog.V(3).Infof("Resource identified as '%s', serving using kubernetes client", m.Kind)
		return f.KubeClient(c, m)
	default:
		return nil, fmt.Errorf("Unable to acquire client for %v", m.Kind)
	}
}

// MultiDescriber provides descriptive informations about both Origin and
// Kubernetes types.
func (f *OriginFactory) MultiDescriber(cmd *cobra.Command, m *kmeta.RESTMapping) (kubectl.Describer, error) {
	mapper, ok := f.Factory.Mapper.(meta.MultiRESTMapper)
	if !ok {
		return nil, fmt.Errorf("the factory mapper must be MultiRESTMapper")
	}
	switch mapper.APINameForResource(m.APIVersion, m.Kind) {
	case meta.OriginAPI:
		return f.OriginDescriber(cmd, m)
	case meta.KubernetesAPI:
		return f.KubeDescriber(cmd, m)
	default:
		return nil, fmt.Errorf("don't know how to describe type %s", m.Kind)
	}
}

// OriginClient provides the REST client for all Origin types
func (f *OriginFactory) OriginClient(c *cobra.Command, m *kmeta.RESTMapping) (*client.Client, error) {
	return client.New(kubecmd.GetKubeConfig(c))
}

// RESTHelper provides a REST client for kubectl commands
func (f *OriginFactory) RESTHelper(cmd *cobra.Command) func(*kmeta.RESTMapping) (*kubectl.RESTHelper, error) {
	return func(mapping *kmeta.RESTMapping) (*kubectl.RESTHelper, error) {
		c, err := f.Client(cmd, mapping)
		return kubectl.NewRESTHelper(c, mapping), err
	}
}

// AddCommands registers Kubernetes kubectl commands and also Origin kubectl
// commands.
// TODO: There should be a function upstream that we can call here which
// provides all Kubernetes commands. In meanwhile, this list will have to
// be maintained manually.
func (f *OriginFactory) AddCommands(cmds *cobra.Command, out io.Writer) {
	// Kubernetes commands
	cmds.AddCommand(kubecmd.NewCmdProxy(out))
	cmds.AddCommand(f.Factory.NewCmdGet(out))
	cmds.AddCommand(f.Factory.NewCmdDescribe(out))
	cmds.AddCommand(f.Factory.NewCmdCreate(out))
	cmds.AddCommand(f.Factory.NewCmdUpdate(out))
	cmds.AddCommand(f.Factory.NewCmdDelete(out))
	cmds.AddCommand(f.Factory.NewCmdCreateAll(out))
	cmds.AddCommand(kubecmd.NewCmdNamespace(out))
	cmds.AddCommand(kubecmd.NewCmdLog(out))
	// Origin commands
	cmds.AddCommand(f.NewCmdApply(out))
	cmds.AddCommand(f.NewCmdProcess(out))
	cmds.AddCommand(f.NewCmdBuildLogs(out))
	cmds.AddCommand(f.NewCmdStartBuild(out))
}

// NewFactory initialize the kubectl Factory that supports both Kubernetes and
// Origin objects.
func NewOriginFactory() *OriginFactory {
	f := OriginFactory{Factory: kubecmd.NewFactory()}

	// Save the original Kubernetes clientFunc
	f.KubeClient = f.Factory.Client
	f.KubeDescriber = f.Factory.Describer

	// Replace Mapper, Typer and Client for kubectl Factory to support Origin
	// objects
	f.Factory.Mapper = latest.RESTMapper
	f.Factory.Typer = kapi.Scheme

	// Use MultiRESTClient as a default REST client for this Factory
	f.Factory.Client = f.MultiRESTClient
	f.Factory.Describer = f.MultiDescriber
	return &f
}
