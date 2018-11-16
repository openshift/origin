package controllercmd

import (
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/util/logs"
	"k8s.io/client-go/rest"

	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/library-go/pkg/serviceability"
)

var configScheme = runtime.NewScheme()

func init() {
	utilruntime.Must(operatorv1alpha1.Install(configScheme))
}

type StartFunc = controllercmd.StartFunc

type ControllerFunc func(uncastConfig runtime.Object) (StartFunc, *operatorv1alpha1.GenericOperatorConfig, error)

type ControllerCommandConfig struct {
	componentName string
	version       version.Info
	flags         *controllercmd.ControllerFlags

	componentNamespace string

	controllerFunc ControllerFunc

	configType   reflect.Type
	configScheme *runtime.Scheme
	versions     []schema.GroupVersion
}

func NewControllerCommandConfig(componentName string, version version.Info) *ControllerCommandConfig {
	c := &ControllerCommandConfig{
		componentName: componentName,
		version:       version,
		flags:         controllercmd.NewControllerFlags(),
	}
	return c.WithConfig(&operatorv1alpha1.GenericOperatorConfig{}, configScheme, operatorv1alpha1.GroupVersion)
}

func (c *ControllerCommandConfig) WithStartFunc(startFunc controllercmd.StartFunc) *ControllerCommandConfig {
	controllerFunc := func(uncastConfig runtime.Object) (controllercmd.StartFunc, *operatorv1alpha1.GenericOperatorConfig, error) {
		operatorConfig := uncastConfig.(*operatorv1alpha1.GenericOperatorConfig)
		return startFunc, operatorConfig, nil
	}
	return c.WithControllerFunc(controllerFunc)
}

func (c *ControllerCommandConfig) WithControllerFunc(controllerFunc ControllerFunc) *ControllerCommandConfig {
	c.controllerFunc = controllerFunc
	return c
}

func (c *ControllerCommandConfig) WithNamespace(componentNamespace string) *ControllerCommandConfig {
	c.componentNamespace = componentNamespace
	return c
}

func (c *ControllerCommandConfig) WithConfig(config runtime.Object, configScheme *runtime.Scheme, versions ...schema.GroupVersion) *ControllerCommandConfig {
	c.configType = reflect.TypeOf(config)
	c.configScheme = configScheme
	c.versions = versions
	return c
}

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

			if err := c.flags.Validate(); err != nil {
				glog.Fatal(err)
			}

			if err := c.startController(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	c.flags.AddFlags(cmd)

	return cmd
}

func (c *ControllerCommandConfig) startController() error {
	uncastConfig, err := c.flags.ToConfigObj(c.configScheme, c.versions...)
	if err != nil {
		return err
	}

	if reflect.TypeOf(uncastConfig) != c.configType {
		return fmt.Errorf("unexpected config type: %T, expected: %s", uncastConfig, c.configType.String())
	}

	startFunc, config, err := c.controllerFunc(uncastConfig)
	if err != nil {
		return err
	}

	injectComponentNameFunc := func(config *rest.Config, stop <-chan struct{}) error {
		newConfig := rest.CopyConfig(config)
		newConfig.UserAgent = c.componentName + " " + rest.DefaultKubernetesUserAgent()
		return startFunc(newConfig, stop)
	}

	return controllercmd.NewController(c.componentName, injectComponentNameFunc).
		WithKubeConfigFile(c.flags.KubeConfigFile, nil).
		WithLeaderElection(config.LeaderElection, c.componentNamespace, c.componentName+"-lock").
		Run()
}
