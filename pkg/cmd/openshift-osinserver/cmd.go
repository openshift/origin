package openshift_osinserver

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
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	configv1 "github.com/openshift/api/config/v1"
	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	osinv1 "github.com/openshift/api/osin/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/openshift/origin/pkg/api/legacy"
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/configdefault"
	"github.com/openshift/origin/pkg/configconversion"
)

type OpenShiftOsinServer struct {
	ConfigFile string
}

func NewOpenShiftOsinServer(out, errout io.Writer, stopCh <-chan struct{}) *cobra.Command {
	options := &OpenShiftOsinServer{}

	cmd := &cobra.Command{
		Use:   "openshift-osinserver",
		Short: "Launch OpenShift osin server",
		Run: func(c *cobra.Command, args []string) {
			legacy.InstallInternalLegacyAll(legacyscheme.Scheme)

			kcmdutil.CheckErr(options.Validate())

			serviceability.StartProfiler()

			if err := options.RunOsinServer(stopCh); err != nil {
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
	flags.StringVar(&options.ConfigFile, "config", "", "Location of the osin configuration file to run from.")
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

func (o *OpenShiftOsinServer) RunOsinServer(stopCh <-chan struct{}) error {
	// try to decode into our new types first.  right now there is no validation, no file path resolution.  this unsticks the operator to start.
	// TODO add those things
	configContent, err := ioutil.ReadFile(o.ConfigFile)
	if err != nil {
		return err
	}

	// TODO this probably needs to be updated to a container inside openshift/api/osin/v1
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

	// TODO this is our pretend OsinServerConfig
	config := obj.(*kubecontrolplanev1.KubeAPIServerConfig)
	if err := helpers.ResolvePaths(configconversion.GetKubeAPIServerConfigFileReferences(config), configFileLocation); err != nil {
		return err
	}
	configdefault.SetRecommendedKubeAPIServerConfigDefaults(config)

	return RunOpenShiftOsinServer(config, stopCh)
}
