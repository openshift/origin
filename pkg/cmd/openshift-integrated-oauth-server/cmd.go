package openshift_integrated_oauth_server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	configv1 "github.com/openshift/api/config/v1"
	osinv1 "github.com/openshift/api/osin/v1"
	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/openshift/origin/pkg/api/legacy"
)

type OsinServer struct {
	ConfigFile string
}

func NewOsinServer(out, errout io.Writer, stopCh <-chan struct{}) *cobra.Command {
	options := &OsinServer{}

	cmd := &cobra.Command{
		Use:   "osinserver",
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

func (o *OsinServer) Validate() error {
	if len(o.ConfigFile) == 0 {
		return errors.New("--config is required for this command")
	}

	return nil
}

func (o *OsinServer) RunOsinServer(stopCh <-chan struct{}) error {
	configContent, err := ioutil.ReadFile(o.ConfigFile)
	if err != nil {
		return err
	}

	// TODO this probably needs to be updated to a container inside openshift/api/osin/v1
	scheme := runtime.NewScheme()
	utilruntime.Must(osinv1.Install(scheme))
	codecs := serializer.NewCodecFactory(scheme)
	obj, err := runtime.Decode(codecs.UniversalDecoder(osinv1.GroupVersion, configv1.GroupVersion), configContent)
	if err != nil {
		// TODO drop this code once we remove the hypershift path
		obj = &osinv1.OsinServerConfig{}
		if jsonErr := json.Unmarshal(configContent, obj); jsonErr != nil {
			glog.Errorf("osin config parse error: %v", jsonErr)
			return err
		}
		// return err
	}

	config, ok := obj.(*osinv1.OsinServerConfig)
	if !ok {
		return fmt.Errorf("expected OsinServerConfig, got %T", config)
	}

	return RunOsinServer(config, stopCh)
}
