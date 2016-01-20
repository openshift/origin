package config

import (
	"io/ioutil"

	"encoding/json"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// DefaultConfigPath specifies the default location of the STI config file
const DefaultConfigPath = ".stifile"

// Config represents a basic serialization for the STI build options
type Config struct {
	Source       string            `json:"source" yaml:"source"`
	BuilderImage string            `json:"builderImage" yaml:"builderImage"`
	Tag          string            `json:"tag,omitempty" yaml:"tag,omitempty"`
	Flags        map[string]string `json:"flags,omitempty" yaml:"flags,omitempty"`
}

// Save persists the STI command line arguments into disk
func Save(config *api.Config, cmd *cobra.Command) {
	c := Config{
		BuilderImage: config.BuilderImage,
		Source:       config.Source,
		Tag:          config.Tag,
		Flags:        make(map[string]string),
	}
	// Store only flags that have changed
	cmd.Flags().Visit(func(f *pflag.Flag) {
		c.Flags[f.Name] = f.Value.String()
	})
	data, err := json.Marshal(c)
	if err != nil {
		glog.V(1).Infof("Unable to serialize to %s: %v", DefaultConfigPath, err)
		return
	}
	if err := ioutil.WriteFile(DefaultConfigPath, data, 0644); err != nil {
		glog.V(1).Infof("Unable to save %s: %v", DefaultConfigPath, err)
	}
	return
}

// Restore loads the arguments from disk and prefills the Request
func Restore(config *api.Config, cmd *cobra.Command) {
	data, err := ioutil.ReadFile(DefaultConfigPath)
	if err != nil {
		glog.V(1).Infof("Unable to restore %s: %v", DefaultConfigPath, err)
		return
	}
	c := Config{}
	if err := json.Unmarshal(data, &c); err != nil {
		glog.V(1).Infof("Unable to parse %s: %v", DefaultConfigPath, err)
		return
	}
	config.BuilderImage = c.BuilderImage
	config.Source = c.Source
	config.Tag = c.Tag
	for name, value := range c.Flags {
		// Do not change flags that user sets. Allow overriding of stored flags.
		if cmd.Flag(name).Changed {
			continue
		}
		cmd.Flags().Set(name, value)
	}
}
