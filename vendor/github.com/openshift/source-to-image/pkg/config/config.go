package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/scm/git"
	utillog "github.com/openshift/source-to-image/pkg/util/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var log = utillog.StderrLog
var savedEnvMatcher = regexp.MustCompile("env-[0-9]+")

// DefaultConfigPath specifies the default location of the S2I config file
const DefaultConfigPath = ".s2ifile"

// Config represents a basic serialization for the S2I build options.
type Config struct {
	Source       string            `json:"source" yaml:"source"`
	BuilderImage string            `json:"builderImage" yaml:"builderImage"`
	Tag          string            `json:"tag,omitempty" yaml:"tag,omitempty"`
	Flags        map[string]string `json:"flags,omitempty" yaml:"flags,omitempty"`
}

// Save persists the S2I command line arguments to disk.
func Save(config *api.Config, cmd *cobra.Command) {
	c := Config{
		BuilderImage: config.BuilderImage,
		Source:       config.Source.String(),
		Tag:          config.Tag,
		Flags:        make(map[string]string),
	}
	cmd.Flags().Visit(func(f *pflag.Flag) {
		if f.Name == "env" {
			for i, env := range config.Environment {
				c.Flags[fmt.Sprintf("%s-%d", f.Name, i)] = fmt.Sprintf("%s=%s", env.Name, env.Value)
			}
		} else {
			c.Flags[f.Name] = f.Value.String()
		}
	})
	data, err := json.Marshal(c)
	if err != nil {
		log.V(1).Infof("Unable to serialize to %s: %v", DefaultConfigPath, err)
		return
	}
	if err := ioutil.WriteFile(DefaultConfigPath, data, 0644); err != nil {
		log.V(1).Infof("Unable to save %s: %v", DefaultConfigPath, err)
	}
	return
}

// Restore loads the arguments from disk and prefills the Request
func Restore(config *api.Config, cmd *cobra.Command) {
	data, err := ioutil.ReadFile(DefaultConfigPath)
	if err != nil {
		data, err = ioutil.ReadFile(".stifile")
		if err != nil {
			log.V(1).Infof("Unable to restore %s: %v", DefaultConfigPath, err)
			return
		}
		log.Infof("DEPRECATED: Use %s instead of .stifile", DefaultConfigPath)
	}

	c := Config{}
	if err := json.Unmarshal(data, &c); err != nil {
		log.V(1).Infof("Unable to parse %s: %v", DefaultConfigPath, err)
		return
	}

	source, err := git.Parse(c.Source)
	if err != nil {
		log.V(1).Infof("Unable to parse %s: %v", c.Source, err)
		return
	}

	config.BuilderImage = c.BuilderImage
	config.Source = source
	config.Tag = c.Tag

	envOverride := false
	if cmd.Flag("env").Changed {
		envOverride = true
	}

	for name, value := range c.Flags {
		// Do not change flags that user sets. Allow overriding of stored flags.
		if name == "env" {
			if envOverride {
				continue
			}
			for _, v := range strings.Split(value, ",") {
				cmd.Flags().Set(name, v)
			}
		} else if savedEnvMatcher.MatchString(name) {
			if envOverride {
				continue
			}
			cmd.Flags().Set("env", value)
		} else {
			if cmd.Flag(name).Changed {
				continue
			}
			cmd.Flags().Set(name, value)
		}
	}
}
