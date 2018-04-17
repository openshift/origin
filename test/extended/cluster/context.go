package cluster

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const (
	IF_EXISTS_DELETE = "delete"
	IF_EXISTS_REUSE  = "reuse"
)

// ContextType is the root config struct
type ContextType struct {
	ClusterLoader struct {
		Cleanup    bool
		Projects   []ClusterLoaderType
		Sync       SyncObjectType  `yaml:",omitempty"`
		TuningSets []TuningSetType `yaml:",omitempty"`
	}
}

// ClusterLoaderType struct only used for Cluster Loader test config
type ClusterLoaderType struct {
	Number     int `mapstructure:"num" yaml:"num"`
	Basename   string
	IfExists   string                    `json:"ifexists"`
	Labels     map[string]string         `yaml:",omitempty"`
	Tuning     string                    `yaml:",omitempty"`
	Configmaps map[string]interface{}    `yaml:",omitempty"`
	Secrets    map[string]interface{}    `yaml:",omitempty"`
	Pods       []ClusterLoaderObjectType `yaml:",omitempty"`
	Templates  []ClusterLoaderObjectType `yaml:",omitempty"`
}

// ClusterLoaderObjectType is nested object type for cluster loader struct
type ClusterLoaderObjectType struct {
	Total      int                    `yaml:",omitempty"`
	Number     int                    `mapstructure:"num" yaml:"num"`
	Image      string                 `yaml:",omitempty"`
	Basename   string                 `yaml:",omitempty"`
	File       string                 `yaml:",omitempty"`
	Sync       SyncObjectType         `yaml:",omitempty"`
	Parameters map[string]interface{} `yaml:",omitempty"`
}

// SyncObjectType is nested object type for cluster loader synchronisation functionality
type SyncObjectType struct {
	Server struct {
		Enabled bool
		Port    int
	}
	Running   bool
	Succeeded bool
	Selectors map[string]string
	Timeout   string
}

// TuningSetType is nested type for controlling Cluster Loader deployment pattern
type TuningSetType struct {
	Name      string
	Pods      TuningSetObjectType
	Templates TuningSetObjectType
}

// TuningSetObjectType is shared struct for Pods & Templates
type TuningSetObjectType struct {
	Stepping struct {
		StepSize int
		Pause    time.Duration
		Timeout  time.Duration
	}
	RateLimit struct {
		Delay time.Duration
	}
}

// ConfigContext variable of type ContextType
var ConfigContext ContextType

// PodCount struct keeps HTTP requst counts and state
type PodCount struct {
	Started  int
	Stopped  int
	Shutdown chan bool
}

// ServiceInfo struct to bundle env data
type ServiceInfo struct {
	Name string
	IP   string
	Port int32
}

// ParseConfig will complete flag parsing as well as viper tasks
func ParseConfig(config string, isFixture bool) error {
	// This must be done after common flags are registered, since Viper is a flag option.
	if isFixture {
		dir, file := filepath.Split(config)
		s := strings.Split(file, ".")
		viper.SetConfigName(s[0])
		viper.AddConfigPath(dir)
	} else {
		viper.SetConfigName(config)
		viper.AddConfigPath(".")
	}
	err := viper.ReadInConfig()
	if err != nil {
		return err
	}
	viper.Unmarshal(&ConfigContext)
	return nil
}
