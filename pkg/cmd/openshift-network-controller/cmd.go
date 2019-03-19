package openshift_network_controller

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/openshift/origin/pkg/cmd/openshift-controller-manager/configdefault"
	"k8s.io/client-go/rest"

	"github.com/coreos/go-systemd/daemon"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/clientcmd/api"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	configv1 "github.com/openshift/api/config/v1"
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/openshift/origin/pkg/configconversion"
)

const RecommendedStartNetworkControllerName = "openshift-network-controller"

type OpenShiftNetworkController struct {
	ConfigFilePath string
	Output         io.Writer
}

var longDescription = templates.LongDesc(`
	Start the OpenShift SDN controller`)

func NewOpenShiftNetworkControllerCommand(name, basename string, out, errout io.Writer) *cobra.Command {
	options := &OpenShiftNetworkController{Output: out}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Start the OpenShift SDN controller",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			rest.CommandNameOverride = name
			kcmdutil.CheckErr(options.Validate())

			serviceability.StartProfiler()

			if err := options.StartNetworkController(); err != nil {
				if kerrors.IsInvalid(err) {
					if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
						fmt.Fprintf(errout, "Invalid %s %s\n", details.Kind, details.Name)
						for _, cause := range details.Causes {
							fmt.Fprintf(errout, "  %s: %s\n", cause.Field, cause.Message)
						}
						os.Exit(255)
					}
				}
				glog.Fatal(err)
			}
		},
	}

	flags := cmd.Flags()
	// This command only supports reading from config
	flags.StringVar(&options.ConfigFilePath, "config", options.ConfigFilePath, "Location of the master configuration file to run from.")
	cmd.MarkFlagFilename("config", "yaml", "yml")
	cmd.MarkFlagRequired("config")

	return cmd
}

func (o *OpenShiftNetworkController) Validate() error {
	if len(o.ConfigFilePath) == 0 {
		return errors.New("--config is required for this command")
	}

	return nil
}

// StartNetworkController calls RunNetworkController and then waits forever
func (o *OpenShiftNetworkController) StartNetworkController() error {
	if err := o.RunNetworkController(); err != nil {
		return err
	}

	go daemon.SdNotify(false, "READY=1")
	select {}
}

// RunNetworkController takes the options and starts the network controller
func (o *OpenShiftNetworkController) RunNetworkController() error {
	// try to decode into our new types first.  right now there is no validation, no file path resolution.  this unsticks the operator to start.
	// TODO add those things
	configContent, err := ioutil.ReadFile(o.ConfigFilePath)
	if err != nil {
		return err
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(openshiftcontrolplanev1.Install(scheme))
	codecs := serializer.NewCodecFactory(scheme)
	obj, err := runtime.Decode(codecs.UniversalDecoder(openshiftcontrolplanev1.GroupVersion, configv1.GroupVersion), configContent)
	if err != nil {
		return err
	}

	// Resolve relative to CWD
	absoluteConfigFile, err := api.MakeAbs(o.ConfigFilePath, "")
	if err != nil {
		return err
	}
	configFileLocation := path.Dir(absoluteConfigFile)

	config := obj.(*openshiftcontrolplanev1.OpenShiftControllerManagerConfig)
	/// this isn't allowed to be nil when by itself.
	// TODO remove this when the old path is gone.
	if config.ServingInfo == nil {
		config.ServingInfo = &configv1.HTTPServingInfo{}
	}
	if err := helpers.ResolvePaths(configconversion.GetOpenShiftControllerConfigFileReferences(config), configFileLocation); err != nil {
		return err
	}
	configdefault.SetRecommendedOpenShiftControllerConfigDefaults(config)

	clientConfig, err := helpers.GetKubeClientConfig(config.KubeClientConfig)
	if err != nil {
		return err
	}
	return RunOpenShiftNetworkController(config, clientConfig)
}
