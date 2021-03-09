package monitor

type noOpMonitor struct {
}

func NewNoOpMonitor() Recorder {
	return &noOpMonitor{}
}

func (*noOpMonitor) Record(conditions ...Condition) {}
func (*noOpMonitor) AddSampler(fn SamplerFunc)      {}
