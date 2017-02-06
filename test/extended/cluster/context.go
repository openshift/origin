package cluster

import (
	"time"

	"github.com/spf13/viper"
)

// ContextType is the root config struct
type ContextType struct {
	ClusterLoader struct {
		Cleanup    bool
		Projects   []ClusterLoaderType
		TuningSets []TuningSetType
	}
}

// ClusterLoaderType struct only used for Cluster Loader test config
type ClusterLoaderType struct {
	Number    int `mapstructure:"num"`
	Basename  string
	Tuning    string
	Pods      []ClusterLoaderObjectType
	Templates []ClusterLoaderObjectType
}

// ClusterLoaderObjectType is nested object type for cluster loader struct
type ClusterLoaderObjectType struct {
	Total      int
	Number     int `mapstructure:"num"`
	Image      string
	Basename   string
	File       string
	Parameters ParameterConfigType
}

// ParameterConfigType contains config parameters for each object
type ParameterConfigType struct {
	Run         string `mapstructure:"run"`
	RouterIP    string `mapstructure:"router_ip"`
	TargetHost  string `mapstructure:"target_host"`
	DurationSec int    `mapstructure:"duration"`
	Megabytes   int
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

// TestResult struct contains result data to be saved at end of run
type TestResult struct {
	Time time.Duration `json:"time"`
}

// ParseConfig will complete flag parsing as well as viper tasks
func ParseConfig(config string) {
	// This must be done after common flags are registered, since Viper is a flag option.
	viper.SetConfigName(config)
	viper.AddConfigPath(".")
	viper.ReadInConfig()
	viper.Unmarshal(&ConfigContext)
}
