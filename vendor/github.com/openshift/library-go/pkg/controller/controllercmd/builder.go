package controllercmd

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/config/client"
	leaderelectionconverter "github.com/openshift/library-go/pkg/config/leaderelection"
)

// StartFunc is the function to call on leader election start
type StartFunc func(config *rest.Config, stop <-chan struct{}) error

// OperatorBuilder allows the construction of an controller in optional pieces.
type ControllerBuilder struct {
	kubeAPIServerConfigFile *string
	clientOverrides         *client.ClientConnectionOverrides
	leaderElection          *configv1.LeaderElection

	startFunc        StartFunc
	componentName    string
	instanceIdentity string

	// TODO add serving info, authentication, and authorization
}

// NewController returns a builder struct for constructing the command you want to run
func NewController(componentName string, startFunc StartFunc) *ControllerBuilder {
	return &ControllerBuilder{
		startFunc:     startFunc,
		componentName: componentName,
	}
}

// WithLeaderElection adds leader election options
func (b *ControllerBuilder) WithLeaderElection(leaderElection configv1.LeaderElection, defaultNamespace, defaultName string) *ControllerBuilder {
	if leaderElection.Disable {
		return b
	}

	defaulted := leaderelectionconverter.LeaderElectionDefaulting(leaderElection, defaultNamespace, defaultName)
	b.leaderElection = &defaulted
	return b
}

// WithKubeConfigFile sets an optional kubeconfig file. inclusterconfig will be used if filename is empty
func (b *ControllerBuilder) WithKubeConfigFile(kubeConfigFilename string, defaults *client.ClientConnectionOverrides) *ControllerBuilder {
	b.kubeAPIServerConfigFile = &kubeConfigFilename
	b.clientOverrides = defaults
	return b
}

// WithInstanceIdentity sets the instance identity to use if you need something special. The default is just a UID which is
// usually fine for a pod.
func (b *ControllerBuilder) WithInstanceIdentity(identity string) *ControllerBuilder {
	b.instanceIdentity = identity
	return b
}

// Run starts your controller for you.  It uses leader election if you asked, otherwise it directly calls you
func (b *ControllerBuilder) Run() error {
	clientConfig, err := b.getClientConfig()
	if err != nil {
		return err
	}

	if b.leaderElection == nil {
		if err := b.startFunc(clientConfig, wait.NeverStop); err != nil {
			return err
		}
		return fmt.Errorf("exited")
	}

	leaderElection, err := leaderelectionconverter.ToConfigMapLeaderElection(clientConfig, *b.leaderElection, b.componentName, b.instanceIdentity)
	if err != nil {
		return err
	}

	leaderElection.Callbacks.OnStartedLeading = func(stop <-chan struct{}) {
		if err := b.startFunc(clientConfig, stop); err != nil {
			glog.Fatal(err)
		}
	}
	leaderelection.RunOrDie(leaderElection)
	return fmt.Errorf("exited")
}

func (b *ControllerBuilder) getClientConfig() (*rest.Config, error) {
	kubeconfig := ""
	if b.kubeAPIServerConfigFile != nil {
		kubeconfig = *b.kubeAPIServerConfigFile
	}

	return client.GetKubeConfigOrInClusterConfig(kubeconfig, b.clientOverrides)
}

func (b *ControllerBuilder) getNamespace() (*rest.Config, error) {
	kubeconfig := ""
	if b.kubeAPIServerConfigFile != nil {
		kubeconfig = *b.kubeAPIServerConfigFile
	}

	return client.GetKubeConfigOrInClusterConfig(kubeconfig, b.clientOverrides)
}
