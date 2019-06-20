package openshift_kube_apiserver

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
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	configv1 "github.com/openshift/api/config/v1"
	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	osinv1 "github.com/openshift/api/osin/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
	"github.com/openshift/library-go/pkg/serviceability"

	"k8s.io/kubernetes/openshift-kube-apiserver/configdefault"
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
			rest.CommandNameOverride = name
			if err := options.Validate(); err != nil {
				klog.Fatal(err)
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
				klog.Fatal(err)
			}
			// When no error is returned, always return with zero exit code.
			// This is here to make sure the container that run apiserver won't get accidentally restarted
			// when the pod runs with restart on failure.
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
	if err != nil {

		return err
	}

	// Resolve relative to CWD
	absoluteConfigFile, err := api.MakeAbs(o.ConfigFile, "")
	if err != nil {
		return err
	}
	configFileLocation := path.Dir(absoluteConfigFile)

	config := obj.(*kubecontrolplanev1.KubeAPIServerConfig)
	if err := helpers.ResolvePaths(GetKubeAPIServerConfigFileReferences(config), configFileLocation); err != nil {
		return err
	}
	configdefault.SetRecommendedKubeAPIServerConfigDefaults(config)
	configdefault.ResolveDirectoriesForSATokenVerification(config)

	return RunOpenShiftKubeAPIServerServer(config, stopCh)

}
