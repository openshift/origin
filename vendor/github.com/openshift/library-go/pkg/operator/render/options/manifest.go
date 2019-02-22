package options

import (
	"errors"
	"fmt"

	"github.com/spf13/pflag"
)

// ManifestOptions contains the values that influence manifest contents.
type ManifestOptions struct {
	Namespace             string
	Image                 string
	ImagePullPolicy       string
	ConfigHostPath        string
	ConfigFileName        string
	CloudProviderHostPath string
	SecretsHostPath       string
}

// NewManifestOptions return default values for ManifestOptions.
func NewManifestOptions(componentName, image string) *ManifestOptions {
	return &ManifestOptions{
		Namespace:             fmt.Sprintf("openshift-%s", componentName),
		Image:                 image,
		ImagePullPolicy:       "IfNotPresent",
		ConfigHostPath:        "/etc/kubernetes/bootstrap-configs",
		ConfigFileName:        fmt.Sprintf("%s-config.yaml", componentName),
		CloudProviderHostPath: "/etc/kubernetes/cloud",
		SecretsHostPath:       "/etc/kubernetes/bootstrap-secrets",
	}
}

// AddfFlags adds the manifest related flags to the flagset.
func (o *ManifestOptions) AddFlags(fs *pflag.FlagSet, humanReadableComponentName string) {
	fs.StringVar(&o.Namespace, "manifest-namespace", o.Namespace,
		fmt.Sprintf("Target namespace for phase 3 %s pods.", humanReadableComponentName))
	fs.StringVar(&o.Image, "manifest-image", o.Image,
		fmt.Sprintf("Image to use for the %s.", humanReadableComponentName))
	fs.StringVar(&o.ImagePullPolicy, "manifest-image-pull-policy", o.ImagePullPolicy,
		fmt.Sprintf("Image pull policy to use for the %s.", humanReadableComponentName))
	fs.StringVar(&o.ConfigHostPath, "manifest-config-host-path", o.ConfigHostPath,
		fmt.Sprintf("A host path mounted into the %s pods to hold a config file.", humanReadableComponentName))
	fs.StringVar(&o.SecretsHostPath, "manifest-secrets-host-path", o.SecretsHostPath,
		fmt.Sprintf("A host path mounted into the %s pods to hold secrets.", humanReadableComponentName))
	fs.StringVar(&o.ConfigFileName, "manifest-config-file-name", o.ConfigFileName,
		"The config file name inside the manifest-config-host-path.")
	fs.StringVar(&o.CloudProviderHostPath, "manifest-cloud-provider-host-path", o.CloudProviderHostPath,
		fmt.Sprintf("A host path mounted into the %s pods to hold cloud provider configuration.", humanReadableComponentName))
}

// Complete fills in missing values before execution.
func (o *ManifestOptions) Complete() error {
	return nil
}

// Validate verifies the inputs.
func (o *ManifestOptions) Validate() error {
	if len(o.Namespace) == 0 {
		return errors.New("missing required flag: --manifest-namespace")
	}
	if len(o.Image) == 0 {
		return errors.New("missing required flag: --manifest-image")
	}
	if len(o.ImagePullPolicy) == 0 {
		return errors.New("missing required flag: --manifest-image-pull-policy")
	}
	if len(o.ConfigHostPath) == 0 {
		return errors.New("missing required flag: --manifest-config-host-path")
	}
	if len(o.ConfigFileName) == 0 {
		return errors.New("missing required flag: --manifest-config-file-name")
	}
	if len(o.CloudProviderHostPath) == 0 {
		return errors.New("missing required flag: --manifest-cloud-provider-host-path")
	}
	if len(o.SecretsHostPath) == 0 {
		return errors.New("missing required flag: --manifest-secrets-host-path")
	}

	return nil
}

// ApplyTo applies the options ot the given config struct.
func (o *ManifestOptions) ApplyTo(cfg *ManifestConfig) error {
	cfg.Namespace = o.Namespace
	cfg.Image = o.Image
	cfg.ImagePullPolicy = o.ImagePullPolicy
	cfg.ConfigHostPath = o.ConfigHostPath
	cfg.ConfigFileName = o.ConfigFileName
	cfg.CloudProviderHostPath = o.CloudProviderHostPath
	cfg.SecretsHostPath = o.SecretsHostPath

	return nil
}
