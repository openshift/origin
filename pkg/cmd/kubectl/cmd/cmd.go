package cmd

import (
	"fmt"
	"io"
	"os"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kmeta "github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	client "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/meta"
	"github.com/spf13/cobra"
)

const OriginAPIPrefix = "/osapi"

// OriginFactory provides a Factory which handles both Kubernetes and Origin
// objects
type OriginFactory struct {
	KubeClientFunc func(*cobra.Command, *kmeta.RESTMapping) (kubectl.RESTClient, error)
	*kubecmd.Factory
}

// MultiRESTClient provides a REST client suitable for managing the given
// Resource Mappings. It determines the right REST client based on the
// MultiRESTMapper.
func (f *OriginFactory) MultiRESTClient(c *cobra.Command, m *kmeta.RESTMapping) (kubectl.RESTClient, error) {
	mapper, ok := f.Factory.Mapper.(meta.MultiRESTMapper)
	if !ok {
		return nil, fmt.Errorf("Mapper '%v' is not a MultiRESTMapper", f.Factory.Mapper)
	}
	switch mapper.APINameForResource(m.APIVersion, m.Kind) {
	case meta.OriginAPI:
		glog.V(3).Infof("Resource identified as '%s', serving using origin client.", m.Kind)
		return f.OriginClientFunc(c, m)
	case meta.KubernetesAPI:
		glog.V(3).Infof("Resource identified as '%s', serving using kubernetes client.", m.Kind)
		return f.KubeClientFunc(c, m)
	default:
		return nil, fmt.Errorf("Unable to acquire client for %v", m.Kind)
	}
}

// OriginClientFunc provides the default REST client for Origin types
func (f *OriginFactory) OriginClientFunc(cmd *cobra.Command, m *kmeta.RESTMapping) (*client.Client, error) {
	config := kubecmd.GetKubeConfig(cmd)

	// Set the OpenShift URI prefix
	config.Prefix = OriginAPIPrefix

	// Set the version for OpenShift object to latest version
	config.Version = latest.Version

	if c, err := client.RESTClientFor(config); err != nil {
		return nil, err
	} else {
		return &client.Client{c}, nil
	}
}

// GetRESTHelperFunc provides a helper to generate a client function in kubectl
// commands.
func (f *OriginFactory) GetRESTHelperFunc(cmd *cobra.Command) func(*kmeta.RESTMapping) (*kubectl.RESTHelper, error) {
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
}

// NewFactory initialize the kubectl Factory that supports both Kubernetes and
// Origin objects.
func NewOriginFactory() *OriginFactory {
	f := OriginFactory{Factory: kubecmd.NewFactory()}

	// Save the original Kubernetes clientFunc
	f.KubeClientFunc = f.Factory.Client

	// Replace Mapper, Typer and Client for kubectl Factory to support Origin
	// objects
	f.Factory.Mapper = latest.RESTMapper
	f.Factory.Typer = kapi.Scheme

	// Use MultiRESTClient as a default REST client for this Factory
	f.Factory.Client = f.MultiRESTClient
	return &f
}

// TODO: Remove this when this func will be public in upstream
func usageError(cmd *cobra.Command, format string, args ...interface{}) {
	glog.Errorf(format, args...)
	glog.Errorf("See '%s -h' for help.", cmd.CommandPath())
	os.Exit(1)
}

// TODO: Remove this when this func will be public in upstream
func checkErr(err error) {
	if err != nil {
		glog.Fatalf("%v", err)
	}
}

// TODO: Make this public in upstream
func getOriginNamespace(cmd *cobra.Command) string {
	result := kapi.NamespaceDefault
	if ns := kubecmd.GetFlagString(cmd, "namespace"); len(ns) > 0 {
		result = ns
		glog.V(2).Infof("Using namespace from -ns flag")
	} else {
		nsPath := kubecmd.GetFlagString(cmd, "ns-path")
		nsInfo, err := kubectl.LoadNamespaceInfo(nsPath)
		if err != nil {
			glog.Fatalf("Error loading current namespace: %v", err)
		}
		result = nsInfo.Namespace
	}
	glog.V(2).Infof("Using namespace %s", result)
	return result

}
