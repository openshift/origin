package openshift_kube_apiserver

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"

	configv1 "github.com/openshift/api/config/v1"
	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	osinv1 "github.com/openshift/api/osin/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/configdefault"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation"
	"github.com/openshift/origin/pkg/configconversion"
	"k8s.io/client-go/tools/clientcmd/api"
)

const RecommendedStartAPIServerName = "openshift-kube-apiserver"

type OpenShiftKubeAPIServerServer struct {
	ConfigFile string
	Output     io.Writer
}

var longDescription = templates.LongDesc(`
	Start the extended kube-apiserver with OpenShift security extensions`)

func NewOpenShiftKubeAPIServerServerCommand(name, basename string, out, errout io.Writer, stopCh <-chan struct{}) *cobra.Command {
	options := &OpenShiftKubeAPIServerServer{Output: out}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Start the OpenShift kube-apiserver",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			if err := options.Validate(); err != nil {
				glog.Fatal(err)
			}

			serviceability.StartProfiler()

			if err := options.RunAPIServer(stopCh); err != nil {
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
	flags.StringVar(&options.ConfigFile, "config", "", "Location of the master configuration file to run from.")
	cmd.MarkFlagFilename("config", "yaml", "yml")
	cmd.MarkFlagRequired("config")

	return cmd
}

func (o *OpenShiftKubeAPIServerServer) Validate() error {
	if len(o.ConfigFile) == 0 {
		return errors.New("--config is required for this command")
	}

	return nil
}

// RunAPIServer takes the options, starts the API server and runs until stopCh is closed or the initial listening fails
func (o *OpenShiftKubeAPIServerServer) RunAPIServer(stopCh <-chan struct{}) error {
	// try to decode into our new types first.  right now there is no validation, no file path resolution.  this unsticks the operator to start.
	// TODO add those things
	configContent, err := ioutil.ReadFile(o.ConfigFile)
	if err != nil {
		return err
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(kubecontrolplanev1.Install(scheme))
	codecs := serializer.NewCodecFactory(scheme)
	obj, err := runtime.Decode(codecs.UniversalDecoder(kubecontrolplanev1.GroupVersion, configv1.GroupVersion, osinv1.GroupVersion), configContent)
	if err == nil {
		// Resolve relative to CWD
		absoluteConfigFile, err := api.MakeAbs(o.ConfigFile, "")
		if err != nil {
			return err
		}
		configFileLocation := path.Dir(absoluteConfigFile)

		config := obj.(*kubecontrolplanev1.KubeAPIServerConfig)
		if err := helpers.ResolvePaths(configconversion.GetKubeAPIServerConfigFileReferences(config), configFileLocation); err != nil {
			return err
		}
		configdefault.SetRecommendedKubeAPIServerConfigDefaults(config)

		return RunOpenShiftKubeAPIServerServer(config, stopCh)
	}

	// TODO this code disappears once the kube-core operator switches to external types
	// TODO we will simply run some defaulting code and convert
	// reading internal gives us defaulting that we need for now
	masterConfig, err := configapilatest.ReadAndResolveMasterConfig(o.ConfigFile)
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
	kubeAPIServerConfig, err := configconversion.ConvertMasterConfigToKubeAPIServerConfig(externalMasterConfig.(*legacyconfigv1.MasterConfig))
	if err != nil {
		return err
	}

	return RunOpenShiftKubeAPIServerServer(kubeAPIServerConfig, stopCh)
}
