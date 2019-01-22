package openshift_controller_manager

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/coreos/go-systemd/daemon"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	configv1 "github.com/openshift/api/config/v1"
	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/openshift/origin/pkg/cmd/openshift-controller-manager/configdefault"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation"
	"github.com/openshift/origin/pkg/configconversion"
)

const RecommendedStartControllerManagerName = "openshift-controller-manager"

type OpenShiftControllerManager struct {
	ConfigFilePath string
	Output         io.Writer
}

var longDescription = templates.LongDesc(`
	Start the OpenShift controllers`)

func NewOpenShiftControllerManagerCommand(name, basename string, out, errout io.Writer) *cobra.Command {
	options := &OpenShiftControllerManager{Output: out}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Start the OpenShift controllers",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Validate())

			serviceability.StartProfiler()

			if err := options.StartControllerManager(); err != nil {
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

func (o *OpenShiftControllerManager) Validate() error {
	if len(o.ConfigFilePath) == 0 {
		return errors.New("--config is required for this command")
	}

	return nil
}

// StartControllerManager calls RunControllerManager and then waits forever
func (o *OpenShiftControllerManager) StartControllerManager() error {
	if err := o.RunControllerManager(); err != nil {
		return err
	}

	go daemon.SdNotify(false, "READY=1")
	select {}
}

// RunControllerManager takes the options and starts the controllers
func (o *OpenShiftControllerManager) RunControllerManager() error {
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
	switch {
	case runtime.IsMissingVersion(err): // fall through to legacy master config
	case runtime.IsMissingKind(err): // fall through to legacy master config
	case runtime.IsNotRegisteredError(err): // fall through to legacy master config
	case err != nil:
		return err
	case err == nil:
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
		return RunOpenShiftControllerManager(config, clientConfig)
	}

	masterConfig, err := configapilatest.ReadAndResolveMasterConfig(o.ConfigFilePath)
	if err != nil {
		return err
	}

	validationResults := validation.ValidateMasterConfig(masterConfig, nil)
	if len(validationResults.Warnings) != 0 {
		for _, warning := range validationResults.Warnings {
			glog.Warningf("%v", warning)
		}
	}
	if len(validationResults.Errors) != 0 {
		return kerrors.NewInvalid(configapi.Kind("MasterConfig"), "master-config.yaml", validationResults.Errors)
	}

	// round trip to external
	externalMasterConfig, err := configapi.Scheme.ConvertToVersion(masterConfig, legacyconfigv1.LegacySchemeGroupVersion)
	if err != nil {
		return err
	}
	config := ConvertMasterConfigToOpenshiftControllerConfig(externalMasterConfig.(*legacyconfigv1.MasterConfig))
	clientConfig, err := helpers.GetKubeClientConfig(config.KubeClientConfig)
	if err != nil {
		return err
	}

	return RunOpenShiftControllerManager(config, clientConfig)
}
