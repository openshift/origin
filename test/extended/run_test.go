/*
 * This is a file allowing you to run ginkgo manually.
 */

package extended

import (
	"math/rand"
	"testing"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
)

// init initialize the extended testing suite.
func init() {
	exutil.InitTest()
}

func TestExtended(t *testing.T) {
	rand.Seed(time.Now().UTC().UnixNano())
	exutil.ExecuteTest(t, "Extended")
}
