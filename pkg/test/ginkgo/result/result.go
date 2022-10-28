package result

import (
	"fmt"
	"sync"
	"time"

	"github.com/onsi/ginkgo"
	"k8s.io/kubernetes/test/e2e/framework"
)

var (
	flakeLock   sync.Mutex
	flakeGinkgo string
)

// Flakef records a flake for the currently running Ginkgo test.
func Flakef(format string, options ...interface{}) {
	if _, ok := ginkgo.GlobalSuite().CurrentRunningSpecSummary(); !ok {
		panic("Flakef called outside of a running Ginkgo test")
	}
	fmt.Fprintf(ginkgo.GinkgoWriter, time.Now().Format(time.StampMilli)+": INFO: "+format+"\n", options...)
	framework.Logf(format, options...)

	flakeLock.Lock()
	defer flakeLock.Unlock()
	flakeGinkgo = fmt.Sprintf(format, options...)
}

// HasFlake returns true if the last Ginko test executed in process flaked.
// Invoking this method clears the flake state.
func LastFlake() (string, bool) {
	flakeLock.Lock()
	defer flakeLock.Unlock()
	s := flakeGinkgo
	flakeGinkgo = ""
	return s, len(s) > 0
}
