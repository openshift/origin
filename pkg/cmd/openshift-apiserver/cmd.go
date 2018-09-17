package openshift_apiserver

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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	configv1 "github.com/openshift/api/config/v1"
	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/openshift/origin/pkg/api/legacy"
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/configdefault"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation"
	"github.com/openshift/origin/pkg/configconversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/tools/clientcmd/api"
)

const RecommendedStartAPIServerName = "openshift-apiserver"

type OpenShiftAPIServer struct {
	ConfigFile string
	Output     io.Writer
}

var longDescription = templates.LongDesc(`
	Start an apiserver that contains the OpenShift resources`)

func NewOpenShiftAPIServerCommand(name, basename string, out, errout io.Writer) *cobra.Command {
	options := &OpenShiftAPIServer{Output: out}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch OpenShift apiserver",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			legacy.InstallInternalLegacyAll(legacyscheme.Scheme)

			kcmdutil.CheckErr(options.Validate())

			serviceability.StartProfiler()

			if err := options.StartAPIServer(); err != nil {
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

func (o *OpenShiftAPIServer) Validate() error {
	if len(o.ConfigFile) == 0 {
		return errors.New("--config is required for this command")
	}

	return nil
}

// StartAPIServer calls RunAPIServer and then waits forever
func (o *OpenShiftAPIServer) StartAPIServer() error {
	if err := o.RunAPIServer(); err != nil {
		return err
	}

	go daemon.SdNotify(false, "READY=1")
	select {}
}

// RunAPIServer takes the options and starts the etcd server
func (o *OpenShiftAPIServer) RunAPIServer() error {
	// try to decode into our new types first.  right now there is no validation, no file path resolution.  this unsticks the operator to start.
	// TODO add those things
	configContent, err := ioutil.ReadFile(o.ConfigFile)
	if err != nil {
		return err
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(openshiftcontrolplanev1.Install(scheme))
	codecs := serializer.NewCodecFactory(scheme)
	obj, err := runtime.Decode(codecs.UniversalDecoder(openshiftcontrolplanev1.GroupVersion, configv1.GroupVersion), configContent)
	if err == nil {
		// Resolve relative to CWD
		absoluteConfigFile, err := api.MakeAbs(o.ConfigFile, "")
		if err != nil {
			return err
		}
		configFileLocation := path.Dir(absoluteConfigFile)

		config := obj.(*openshiftcontrolplanev1.OpenShiftAPIServerConfig)
		if err := helpers.ResolvePaths(configconversion.GetOpenShiftAPIServerConfigFileReferences(config), configFileLocation); err != nil {
			return err
		}
		configdefault.SetRecommendedOpenShiftAPIServerConfigDefaults(config)

		return RunOpenShiftAPIServer(config)
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
	openshiftAPIServerConfig, err := configconversion.ConvertMasterConfigToOpenShiftAPIServerConfig(externalMasterConfig.(*legacyconfigv1.MasterConfig))
	if err != nil {
		return err
	}

	return RunOpenShiftAPIServer(openshiftAPIServerConfig)
}
