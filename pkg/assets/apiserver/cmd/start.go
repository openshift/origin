package cmd

import (
	"fmt"
	"io"
	"io/ioutil"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util"

	webconsoleserver "github.com/openshift/origin/pkg/assets/apiserver"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapiinstall "github.com/openshift/origin/pkg/cmd/server/api/install"
	configapivalidation "github.com/openshift/origin/pkg/cmd/server/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

type WebConsoleServerOptions struct {
	// we don't have any storage, so we shouldn't use the recommended options
	Audit    *genericoptions.AuditOptions
	Features *genericoptions.FeatureOptions

	StdOut io.Writer
	StdErr io.Writer

	WebConsoleConfig *configapi.AssetConfig
}

func NewWebConsoleServerOptions(out, errOut io.Writer) *WebConsoleServerOptions {
	o := &WebConsoleServerOptions{
		Audit:    genericoptions.NewAuditOptions(),
		Features: genericoptions.NewFeatureOptions(),

		StdOut: out,
		StdErr: errOut,
	}

	return o
}

func NewCommandStartWebConsoleServer(out, errOut io.Writer, stopCh <-chan struct{}) *cobra.Command {
	o := NewWebConsoleServerOptions(out, errOut)

	cmd := &cobra.Command{
		Use:   "origin-web-console",
		Short: "Launch a web console server",
		Long:  "Launch a web console server",
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(c); err != nil {
				return err
			}
			if err := o.Validate(args); err != nil {
				return err
			}
			if err := o.RunWebConsoleServer(stopCh); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	o.Audit.AddFlags(flags)
	o.Features.AddFlags(flags)
	flags.String("config", "", "filename containing the WebConsoleConfig")

	return cmd
}

func (o WebConsoleServerOptions) Validate(args []string) error {
	if o.WebConsoleConfig == nil {
		return fmt.Errorf("missing config: specify --config")
	}

	validationResults := configapivalidation.ValidateAssetConfig(o.WebConsoleConfig, field.NewPath("config"))
	if len(validationResults.Warnings) != 0 {
		for _, warning := range validationResults.Warnings {
			glog.Warningf("Warning: %v, web console start will continue.", warning)
		}
	}
	if len(validationResults.Errors) != 0 {
		return apierrors.NewInvalid(configapi.Kind("AssetConfig"), "", validationResults.Errors)
	}

	return nil
}

func (o *WebConsoleServerOptions) Complete(cmd *cobra.Command) error {
	configFile := util.GetFlagString(cmd, "config")
	if len(configFile) > 0 {
		content, err := ioutil.ReadFile(configFile)
		if err != nil {
			return err
		}
		configObj, err := runtime.Decode(configCodecs.UniversalDecoder(), content)
		if err != nil {
			return err
		}
		config, ok := configObj.(*configapi.AssetConfig)
		if !ok {
			return fmt.Errorf("unexpected type: %T", configObj)
		}
		o.WebConsoleConfig = config
	}

	return nil
}

func (o WebConsoleServerOptions) Config() (*webconsoleserver.AssetServerConfig, error) {
	serverConfig, err := webconsoleserver.NewAssetServerConfig(*o.WebConsoleConfig)
	if err != nil {
		return nil, err
	}

	if err := o.Audit.ApplyTo(serverConfig.GenericConfig); err != nil {
		return nil, err
	}
	if err := o.Features.ApplyTo(serverConfig.GenericConfig); err != nil {
		return nil, err
	}

	return serverConfig, nil
}

func (o WebConsoleServerOptions) RunWebConsoleServer(stopCh <-chan struct{}) error {
	config, err := o.Config()
	if err != nil {
		return err
	}

	server, err := config.Complete().New(genericapiserver.EmptyDelegate)
	if err != nil {
		return err
	}
	return server.GenericAPIServer.PrepareRun().Run(stopCh)
}

// these are used to set up for reading the config
var (
	configScheme = runtime.NewScheme()
	configCodecs = serializer.NewCodecFactory(configScheme)
)

func init() {
	configapiinstall.AddToScheme(configScheme)
}
