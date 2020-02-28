package pdebug_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lestrrat-go/pdebug/v2"
	"github.com/stretchr/testify/assert"
)

func TestMarker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var buf bytes.Buffer

	var now = time.Unix(0, 0)
	ctx = pdebug.WithID(ctx, "123456789")
	ctx = pdebug.WithClock(ctx, pdebug.ClockFunc(func() time.Time { return now }))
	ctx = pdebug.WithOutput(ctx, &buf)

	func(ctx context.Context) {
		var err error
		g1 := pdebug.Marker(ctx, "Test 1").BindError(&err)
		defer g1.End()

		pdebug.Printf(ctx, "Hello, World test 1")

		g2 := pdebug.Marker(ctx, "Test 2")
		defer g2.End()

		pdebug.Printf(ctx, "Hello, World test 2")
		err = errors.New("test 1 error")
	}(ctx)

	t.Logf("%s", buf.String())

	if pdebug.Enabled && pdebug.Trace {
		const expected = `|DEBUG| 123456789 0.00000 START Test 1
|DEBUG| 123456789 0.00000   Hello, World test 1
|DEBUG| 123456789 0.00000   START Test 2
|DEBUG| 123456789 0.00000     Hello, World test 2
|DEBUG| 123456789 0.00000   END   Test 2(elapsed=0s)
|DEBUG| 123456789 0.00000 END   Test 1(elapsed=0s, error=test 1 error)
`
		if !assert.Equal(t, expected, buf.String()) {
			return
		}
	}
}
