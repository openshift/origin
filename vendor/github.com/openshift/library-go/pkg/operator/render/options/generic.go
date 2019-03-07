package options

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"text/template"

	"github.com/ghodss/yaml"
	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"github.com/spf13/pflag"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GenericOptions contains the generic render command options.
type GenericOptions struct {
	DefaultFile                   string
	BootstrapOverrideFile         string
	PostBootstrapOverrideFile     string
	AdditionalConfigOverrideFiles []string

	ConfigOutputFile string

	TemplatesDir   string
	AssetInputDir  string
	AssetOutputDir string
}

type Template struct {
	FileName string
	Content  []byte
}

// NewGenericOptions returns a default set of generic options.
func NewGenericOptions() *GenericOptions {
	return &GenericOptions{
		TemplatesDir: "/usr/share/bootkube/manifests",
	}
}

// AddFlags adds the generic flags to the flagset.
func (o *GenericOptions) AddFlags(fs *pflag.FlagSet, configGVK schema.GroupVersionKind) {
	fs.StringVar(&o.AssetOutputDir, "asset-output-dir", o.AssetOutputDir, "Output path for rendered manifests.")
	fs.StringVar(&o.AssetInputDir, "asset-input-dir", o.AssetInputDir, "A path to directory with certificates and secrets.")
	fs.StringVar(&o.TemplatesDir, "templates-input-dir", o.TemplatesDir, "A path to a directory with manifest templates.")
	fs.StringSliceVar(&o.AdditionalConfigOverrideFiles, "config-override-files", o.AdditionalConfigOverrideFiles,
		fmt.Sprintf("Additional sparse %s files for customiziation through the installer, merged into the default config in the given order.", gvkOutput{configGVK}))
	fs.StringVar(&o.ConfigOutputFile, "config-output-file", o.ConfigOutputFile, fmt.Sprintf("Output path for the %s yaml file.", gvkOutput{configGVK}))
}

type gvkOutput struct {
	schema.GroupVersionKind
}

func (gvk gvkOutput) String() string {
	return fmt.Sprintf("%s.%s/%s", gvk.GroupVersionKind.Kind, gvk.GroupVersionKind.Group, gvk.GroupVersionKind.Version)
}

// Complete fills in missing values before execution.
func (o *GenericOptions) Complete() error {
	return nil
}

// Validate verifies the inputs.
func (o *GenericOptions) Validate() error {
	if len(o.AssetInputDir) == 0 {
		return errors.New("missing required flag: --asset-input-dir")
	}
	if len(o.AssetOutputDir) == 0 {
		return errors.New("missing required flag: --asset-output-dir")
	}
	if len(o.TemplatesDir) == 0 {
		return errors.New("missing required flag: --templates-dir")
	}
	if len(o.ConfigOutputFile) == 0 {
		return errors.New("missing required flag: --config-output-file")
	}

	return nil
}

// ApplyTo applies the options ot the given config struct using the provides text/template data.
func (o *GenericOptions) ApplyTo(cfg *FileConfig, defaultConfig, bootstrapOverrides, postBootstrapOverrides Template, templateData interface{}, specialCases map[string]resourcemerge.MergeFunc) error {
	var err error

	cfg.BootstrapConfig, err = o.configFromDefaultsPlusOverride(defaultConfig, bootstrapOverrides, templateData, specialCases)
	if err != nil {
		return fmt.Errorf("failed to generate bootstrap config (phase 1): %v", err)
	}

	if cfg.PostBootstrapConfig, err = o.configFromDefaultsPlusOverride(defaultConfig, postBootstrapOverrides, templateData, specialCases); err != nil {
		return fmt.Errorf("failed to generate post-bootstrap config (phase 2): %v", err)
	}

	// load and render templates
	if cfg.Assets, err = assets.LoadFilesRecursively(o.AssetInputDir); err != nil {
		return fmt.Errorf("failed loading assets from %q: %v", o.AssetInputDir, err)
	}

	return nil
}

func (o *GenericOptions) configFromDefaultsPlusOverride(defaultConfig, overrides Template, templateData interface{}, specialCases map[string]resourcemerge.MergeFunc) ([]byte, error) {
	defaultConfigContent, err := renderTemplate(defaultConfig, templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render default config file %q as text/template: %v", defaultConfig.FileName, err)
	}

	overridesContent, err := renderTemplate(overrides, templateData)
	if err != nil {
		return nil, fmt.Errorf("failed to render config override file %q as text/template: %v", overrides.FileName, err)
	}
	configs := [][]byte{defaultConfigContent, overridesContent}
	for _, fname := range o.AdditionalConfigOverrideFiles {
		bs, err := ioutil.ReadFile(fname)
		if err != nil {
			return nil, fmt.Errorf("failed to load config overrides at %q: %v", fname, err)
		}
		overrides, err := renderTemplate(Template{fname, bs}, templateData)
		if err != nil {
			return nil, fmt.Errorf("failed to render config overrides file %q as text/template: %v", fname, err)
		}

		configs = append(configs, overrides)
	}
	mergedConfig, err := resourcemerge.MergeProcessConfig(specialCases, configs...)
	if err != nil {
		return nil, fmt.Errorf("failed to merge configs: %v", err)
	}
	yml, err := yaml.JSONToYAML(mergedConfig)
	if err != nil {
		return nil, err
	}

	return yml, nil
}

func renderTemplate(tpl Template, data interface{}) ([]byte, error) {
	tmpl, err := template.New(tpl.FileName).Parse(string(tpl.Content))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
