package app

import (
	"k8s.io/kubernetes/pkg/apis/componentconfig"
)

func (o *Options) GetConfig() *componentconfig.KubeSchedulerConfiguration {
	return o.config
}

func (o *Options) ReallyApplyDefaults() (err error) {
	o.config, err = o.ApplyDefaults(o.config)
	return err
}
