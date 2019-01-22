package callrules

import (
	"context"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
)

// WithFrameworkID returns a Rule that injects a framework ID to outgoing calls, with the following exceptions:
//   - SUBSCRIBE calls are never modified (schedulers should explicitly construct such calls)
//   - calls are not modified when the detected framework ID is ""
func WithFrameworkID(frameworkID func() string) Rule {
	return func(ctx context.Context, c *scheduler.Call, r mesos.Response, err error, ch Chain) (context.Context, *scheduler.Call, mesos.Response, error) {
		// never overwrite framework ID for subscribe calls; the scheduler must do that part
		if c.GetType() != scheduler.Call_SUBSCRIBE {
			if fid := frameworkID(); fid != "" {
				c2 := *c
				c2.FrameworkID = &mesos.FrameworkID{Value: fid}
				c = &c2
			}
		}
		return ch(ctx, c, r, err)
	}
}
