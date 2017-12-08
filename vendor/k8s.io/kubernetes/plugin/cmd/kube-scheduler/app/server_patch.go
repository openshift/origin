package app


func (o *Options) GetConfig() *componentconfig.KubeSchedulerConfiguration {
	return o.config
}

func (o *Options) ReallyApplyDefaults() (err error) {
	o.config, err = o.ApplyDefaults(o.config)
	return err
}
