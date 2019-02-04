package openshift_apiserver

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/spf13/cobra"
	"k8s.io/klog"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	configv1 "github.com/openshift/api/config/v1"
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/openshift/origin/pkg/api/legacy"
)

const RecommendedStartAPIServerName = "openshift-apiserver"

type OpenShiftAPIServer struct {
	ConfigFile string
	Output     io.Writer
}

var longDescription = templates.LongDesc(`
	Start an apiserver that contains the OpenShift resources`)

func NewOpenShiftAPIServerCommand(name, basename string, out, errout io.Writer, stopCh <-chan struct{}) *cobra.Command {
	options := &OpenShiftAPIServer{Output: out}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Launch OpenShift apiserver",
		Long:  longDescription,
		Run: func(c *cobra.Command, args []string) {
			rest.CommandNameOverride = name

			legacy.InstallInternalLegacyAll(legacyscheme.Scheme)

			kcmdutil.CheckErr(options.Validate())

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
				klog.Fatal(err)
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

// RunAPIServer takes the options, starts the API server and waits until stopCh is closed or initial listening fails.
func (o *OpenShiftAPIServer) RunAPIServer(stopCh <-chan struct{}) error {
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
	if err != nil {
		return err
	}

	// Resolve relative to CWD
	absoluteConfigFile, err := api.MakeAbs(o.ConfigFile, "")
	if err != nil {
		return err
	}
	configFileLocation := path.Dir(absoluteConfigFile)

	config := obj.(*openshiftcontrolplanev1.OpenShiftAPIServerConfig)
	if err := helpers.ResolvePaths(getOpenShiftAPIServerConfigFileReferences(config), configFileLocation); err != nil {
		return err
	}
	setRecommendedOpenShiftAPIServerConfigDefaults(config)

	return RunOpenShiftAPIServer(config, stopCh)
}
