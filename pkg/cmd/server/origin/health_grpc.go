package origin

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var GRPCThreadLimit = 0

func init() {
	val := os.Getenv("OPENSHIFT_GRPC_LIMIT")
	if len(val) == 0 {
		return
	}
	limit, err := strconv.ParseInt(val, 10, 32)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("illegal grpc limit %q: %v", val, err))
	}
	GRPCThreadLimit = int(limit)
}

func NewGRPCStuckThreads() *TooManyThreadsStuckHealth {
	return &TooManyThreadsStuckHealth{
		name:           "too-many-gprc-connections",
		targetFunction: "google.golang.org/grpc/transport.(*http2Client).NewStream",
		limit:          GRPCThreadLimit,
	}
}

func PanicOnGRPCStuckThreads(interval time.Duration, stopCh <-chan struct{}) {
	stuckThreads := NewGRPCStuckThreads()
	go func() {
		for {
			// catch panics.  depending on the handler, we may hit this again. Imagine sending it to sentry over and over
			// alternatively, this give us the power to "really crash" or "crash after".
			func() {
				utilruntime.HandleCrash()

				if err := stuckThreads.Check(nil); err != nil {
					panic(err)
				}

			}()

			select {
			case <-time.After(interval):
			case <-stopCh:
				return
			}
		}

	}()
}

// TooManyThreadsStuckHealth is a health checker that indicates when we have too many thread in a particular method.
// This condition usually indicates that we got stuck and we should restart ourselves
type TooManyThreadsStuckHealth struct {
	name           string
	targetFunction string
	limit          int
}

func (h *TooManyThreadsStuckHealth) Name() string {
	return "too-many-grpc-connections"
}

func (h *TooManyThreadsStuckHealth) Check(req *http.Request) error {
	if count := h.Count(); count > h.limit {
		return fmt.Errorf("found %d gofuncs in %q; limit %d", count, h.targetFunction, h.limit)
	}
	return nil
}

func (h *TooManyThreadsStuckHealth) Count() int {
	// Find out how many records there are (fetch(nil)),
	// allocate that many records, and get the data.
	// There's a race—more records might be added between
	// the two calls—so allocate a few extra records for safety
	// and also try again if we're very unlucky.
	// The loop should only execute one iteration in the common case.
	var p []goruntime.StackRecord
	n, ok := goruntime.GoroutineProfile(nil)
	for {
		// Allocate room for a slightly bigger profile,
		// in case a few more entries have been added
		// since the call to ThreadProfile.
		p = make([]goruntime.StackRecord, n+10)
		n, ok = goruntime.GoroutineProfile(p)
		if ok {
			p = p[0:n]
			break
		}
		// Profile grew; try again.
	}
	pp := processProfile(p)

	return pp.countFunc(h.targetFunction)
}

func hasFunctionName(stk []uintptr, n string) bool {
	frames := goruntime.CallersFrames(stk)
	for {
		frame, more := frames.Next()
		name := frame.Function
		if strings.Contains(name, n) {
			return true
		}
		if !more {
			break
		}
	}
	return false
}

func processProfile(p runtimeProfile) processedProfile {
	// Build count of each stack.
	var buf bytes.Buffer
	key := func(stk []uintptr) string {
		buf.Reset()
		fmt.Fprintf(&buf, "@")
		for _, pc := range stk {
			fmt.Fprintf(&buf, " %#x", pc)
		}
		return buf.String()
	}
	count := map[string]int{}
	index := map[string]int{}
	var keys []string
	n := p.Len()
	for i := 0; i < n; i++ {
		k := key(p.Stack(i))
		if count[k] == 0 {
			index[k] = i
			keys = append(keys, k)
		}
		count[k]++
	}

	pp := processedProfile{
		keys:  keys,
		count: count,
		index: index,
		p:     p,
	}
	return pp
}

type runtimeProfile []goruntime.StackRecord

func (p runtimeProfile) Len() int              { return len(p) }
func (p runtimeProfile) Stack(i int) []uintptr { return p[i].Stack() }

// processedProfile sorts keys with higher counts first, breaking ties by key string order.
type processedProfile struct {
	keys  []string
	count map[string]int
	index map[string]int
	p     runtimeProfile
}

func (pp *processedProfile) countFunc(f string) int {
	count := 0
	for _, k := range pp.keys {
		if hasFunctionName(pp.p.Stack(pp.index[k]), f) {
			count += pp.count[k]
		}
	}
	return count
}
