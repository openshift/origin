package backenddisruption

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

type Recorder interface {
	StartInterval(t time.Time, condition monitorapi.Condition) int
	EndInterval(startedInterval int, t time.Time)
}
