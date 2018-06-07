package controllercmd

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/util/logs"

	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"github.com/openshift/library-go/pkg/serviceability"
)

var (
	configScheme = runtime.NewScheme()
)

func init() {
	if err := operatorv1alpha1.AddToScheme(configScheme); err != nil {
		panic(err)
	}
}

// ControllerCommandConfig holds values required to construct a command to run.
type ControllerCommandConfig struct {
	componentName string
	startFunc     StartFunc
	version       version.Info

	basicFlags *ControllerFlags
}

// NewControllerConfig returns a new ControllerCommandConfig which can be used to wire up all the boiler plate of a controller
// TODO add more methods around wiring health checks and the like
func NewControllerCommandConfig(componentName string, version version.Info, startFunc StartFunc) *ControllerCommandConfig {
	return &ControllerCommandConfig{
		startFunc:     startFunc,
		componentName: componentName,
		version:       version,

		basicFlags: NewControllerFlags(),
	}
}

// NewCommand returns a new command that a caller must set the Use and Descriptions on.  It wires default log, profiling,
// leader election and other "normal" behaviors.
func (c *ControllerCommandConfig) NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Run: func(cmd *cobra.Command, args []string) {
			// boiler plate for the "normal" command
			rand.Seed(time.Now().UTC().UnixNano())
			logs.InitLogs()
			defer logs.FlushLogs()
			defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"), c.version)()
			defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()
			serviceability.StartProfiler()

			if err := c.basicFlags.Validate(); err != nil {
				glog.Fatal(err)
			}

			if err := c.StartController(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	c.basicFlags.AddFlags(cmd)

	return cmd
}

// StartController runs the controller
func (c *ControllerCommandConfig) StartController() error {
	uncastConfig, err := c.basicFlags.ToConfigObj(configScheme, operatorv1alpha1.SchemeGroupVersion)
	if err != nil {
		return err
	}
	config, ok := uncastConfig.(*operatorv1alpha1.GenericOperatorConfig)
	if !ok {
		return fmt.Errorf("unexpected config: %T", uncastConfig)
	}

	return NewController(c.componentName, c.startFunc).
		WithKubeConfigFile(c.basicFlags.KubeConfigFile, nil).
		WithLeaderElection(config.LeaderElection, "", c.componentName+"-lock").
		Run()
}
