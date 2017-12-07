package app


func (o *Options) ReallyApplyDefaults() (err error) {
	o.config, err = o.ApplyDefaults(o.config)
	return err
}
