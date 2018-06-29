package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DockerRegistryConfiguration holds the necessary configuration options for the docker registry
// All the settings (except for environment variales) will be used to generate configuration file for the
// registry app.
// The struct doesn't attempt to capture all the possible options since there are two many and individual
// storage drivers have their own configuration. It provides just the essential options. Everything else can
// be configured via RawConfig yaml string or via environment variables.
type DockerRegistryConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	// Highest priority configuration.
	// TODO: use an array of EnvVars (see Container.Env at
	// vendor/k8s.io/kubernetes/staging/src/k8s.io/api/core/v1/types.go)
	// TODO: since this doesn't contribute to the resulting config file, move it directly under the Spec?
	Envs map[string]string `protobuf:"bytes,1,rep,name=envs" json:"envs,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`

	// Raw yaml string that will be passed to the configuration file. Second to environment variables in
	// priority. The other settings in this structure will be merged with this string and act as the defaults.
	RawConfig string `json:"rawConfig" protobuf:"bytes,2,opt,name=raw_config,json=rawConfig"`

	// Allows to configure registry's logging.
	Log LogConfiguration `json:"log" protobuf:"bytes,3,opt,name=log"`

	// Allows to configure pullthrough and mirror behaviour.
	Pullthrough *PullthroughConfiguration `json:"pullthrough" protobuf:"bytes,4,opt,name=pullthrough"`
}

// LogConfiguration allows to configure registry's logging.
type LogConfiguration struct {
	Level string `json:"level" protobuf:"bytes,1,opt,name=level"`
}

// PullthroughConfiguration allows to configure pullthrough.
type PullthroughConfiguration struct {
	Mirror bool `json:"mirror" protobuf:"varint,1,opt,name=mirror"`
}
