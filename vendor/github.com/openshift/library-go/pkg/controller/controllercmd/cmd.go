package controllercmd

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/util/logs"

	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"github.com/openshift/library-go/pkg/config/configdefaults"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/serviceability"

	// add prometheus metrics
	_ "github.com/openshift/library-go/pkg/controller/metrics"
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

			if err := c.StartController(wait.NeverStop); err != nil {
				glog.Fatal(err)
			}
		},
	}

	c.basicFlags.AddFlags(cmd)

	return cmd
}

// StartController runs the controller
func (c *ControllerCommandConfig) StartController(stopCh <-chan struct{}) error {
	uncastConfig, err := c.basicFlags.ToConfigObj(configScheme, operatorv1alpha1.SchemeGroupVersion)
	if err != nil {
		return err
	}
	config, ok := uncastConfig.(*operatorv1alpha1.GenericOperatorConfig)
	if !ok {
		return fmt.Errorf("unexpected config: %T", uncastConfig)
	}

	// if we don't have any serving cert/key pairs specified and the defaults are not present, generate a self-signed set
	// TODO maybe this should be optional?  It's a little difficult to come up with a scenario where this is worse than nothing though.
	if len(config.ServingInfo.CertFile) == 0 && len(config.ServingInfo.KeyFile) == 0 {
		servingInfoCopy := config.ServingInfo.DeepCopy()
		configdefaults.SetRecommendedHTTPServingInfoDefaults(servingInfoCopy)
		_, keyErr := os.Stat(servingInfoCopy.KeyFile)
		_, certErr := os.Stat(servingInfoCopy.CertFile)
		if os.IsNotExist(keyErr) && os.IsNotExist(certErr) {
			certDir, err := ioutil.TempDir("", "serving-cert-")
			if err != nil {
				return err
			}
			signerName := fmt.Sprintf("%s-signer@%d", c.componentName, time.Now().Unix())
			ca, err := crypto.MakeCA(
				path.Join(certDir, "serving-signer.crt"),
				path.Join(certDir, "serving-signer.key"),
				path.Join(certDir, "serving-signer.serial"),
				signerName,
				0,
			)
			if err != nil {
				return err
			}

			// force the values to be set to where we are writing the certs
			config.ServingInfo.CertFile = path.Join(certDir, "tls.crt")
			config.ServingInfo.KeyFile = path.Join(certDir, "tls.key")
			// nothing can trust this, so we don't really care about hostnames
			_, err = ca.MakeAndWriteServerCert(config.ServingInfo.CertFile, config.ServingInfo.KeyFile, sets.NewString("localhost"), 30)
			if err != nil {
				return err
			}
		}
	}

	return NewController(c.componentName, c.startFunc).
		WithKubeConfigFile(c.basicFlags.KubeConfigFile, nil).
		WithLeaderElection(config.LeaderElection, "", c.componentName+"-lock").
		WithServer(config.ServingInfo, config.Authentication, config.Authorization).
		Run(stopCh)
}
