package zoo

import (
	"github.com/mesos/mesos-go/api/v0/detector"
)

func init() {
	detector.Register("zk://", detector.PluginFactory(func(spec string, options ...detector.Option) (detector.Master, error) {
		return NewMasterDetector(spec, options...)
	}))
}
