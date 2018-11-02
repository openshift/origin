package openshift_osinserver

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	configv1 "github.com/openshift/api/config/v1"
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

type OpenShiftOsinServer struct {
	KubeConfigFile string
	ConfigFile     string
	Output         io.Writer
}

func NewOpenShiftOsinServer(out, errout io.Writer) *cobra.Command {
	options := &OpenShiftOsinServer{Output: out}

	cmd := &cobra.Command{
		Use:   "openshift-osinserver",
		Short: "Launch OpenShift osin server",
		Run: func(c *cobra.Command, args []string) {
			legacy.InstallInternalLegacyAll(legacyscheme.Scheme)

			kcmdutil.CheckErr(options.Validate())

			serviceability.StartProfiler()

			if err := options.RunOsinServer(); err != nil {
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
	flags.StringVar(&options.KubeConfigFile, "kubeconfig", options.KubeConfigFile, "Optional file that uses the kubeconfig instead of the in-cluster config.")
	// This command only supports reading from config
	flags.StringVar(&options.ConfigFile, "config", "", "Location of the master configuration file to run from.")
	cmd.MarkFlagFilename("config", "yaml", "yml")
	cmd.MarkFlagRequired("config")

	return cmd
}

func (o *OpenShiftOsinServer) Validate() error {
	if len(o.ConfigFile) == 0 {
		return errors.New("--config is required for this command")
	}

	return nil
}

func (o *OpenShiftOsinServer) RunOsinServer() error {
	// try to decode into our new types first.  right now there is no validation, no file path resolution.  this unsticks the operator to start.
	// TODO add those things
	configContent, err := ioutil.ReadFile(o.ConfigFile)
	if err != nil {
		return err
	}

	// TODO this probably needs to be updated to a container inside openshift/api/osin/v1
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

		return fmt.Errorf("not yet supported")
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

	// TODO You need real overrides for rate limiting
	kubeClientConfig, err := helpers.GetKubeConfigOrInClusterConfig(o.KubeConfigFile, configv1.ClientConnectionOverrides{QPS: 400, Burst: 400})
	if err != nil {
		return err
	}

	return RunOpenShiftOsinServer(*masterConfig.OAuthConfig, kubeClientConfig, wait.NeverStop)
}
