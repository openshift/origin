package etcdloganalyzer

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// subStringLevel defines a sub-string we'll scan pod log lines for, and the level the resulting
// interval should have. (Info, Warning, Error)
type subStringLevel struct {
	subString string
	level     monitorapi.IntervalLevel
	// key is a short identifier added to the locator so different event types appear on separate
	// lines in the timeline chart instead of overlapping
	key string
}

type etcdLogLine struct {
	Level         string    `json:"level"`
	Timestamp     time.Time `json:"ts"`
	Msg           string    `json:"msg"`
	LocalMemberID string    `json:"local-member-id"`
}
