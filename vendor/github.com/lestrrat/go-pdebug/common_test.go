package pdebug

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setw(ctx *pdctx, w io.Writer) func() {
	oldw := ctx.Writer
	ctx.Writer = w
	return func() { ctx.Writer = oldw }
}

func TestPrintf(t *testing.T) {
	buf := &bytes.Buffer{}
	wg := setw(DefaultCtx, buf)
	defer wg()

	Printf("Hello, World!")

	if Enabled && Trace {
		re := regexp.MustCompile(`\|DEBUG\| \d+\.\d+ Hello, World!\n`)
		if !assert.True(t, re.MatchString(buf.String()), "Simple Printf works") {
			return
		}
	} else {
		if !assert.Equal(t, "", buf.String(), "Simple Printf should be suppressed") {
			return
		}
	}
}

func TestMarker(t *testing.T) {
	buf := &bytes.Buffer{}
	wg := setw(DefaultCtx, buf)
	defer wg()

	f2 := func() (err error) {
		g := Marker("f2").BindError(&err)
		defer g.End()
		Printf("Hello, World!")
		return errors.New("dummy error")
	}

	f1 := func() {
		g := Marker("f1")
		defer g.End()
		f2()
	}

	f1()

	if Enabled && Trace {
		re := regexp.MustCompile(`\|DEBUG\| \d+\.\d+ START f1\n\|DEBUG\| \d+\.\d+   START f2\n\|DEBUG\| \d+\.\d+     Hello, World!\n\|DEBUG\| \d+\.\d+   END f2 \(`)
		if !assert.True(t, re.MatchString(buf.String()), "Markers should work") {
			t.Logf("Expected '%v'", re)
			t.Logf("Actual   '%v'", buf.String())
			return
		}
	} else {
		if !assert.Equal(t, "", buf.String(), "Markers should work") {
			return
		}
	}
}

func TestLegacyMarker(t *testing.T) {
	buf := &bytes.Buffer{}
	wg := setw(DefaultCtx, buf)
	defer wg()

	f2 := func() (err error) {
		g := IPrintf("START f2")
		defer func() {
			if err == nil {
				g.IRelease("END f2")
			} else {
				g.IRelease("END f2: %s", err)
			}
		}()
		Printf("Hello, World!")
		return errors.New("dummy error")
	}

	f1 := func() {
		g := IPrintf("START f1")
		defer g.IRelease("END f1")
		f2()
	}

	f1()

	if Enabled && Trace {
		re := regexp.MustCompile(`\|DEBUG\| \d+\.\d+ START f1\n\|DEBUG\| \d+\.\d+   START f2\n\|DEBUG\| \d+\.\d+     Hello, World!\n\|DEBUG\| \d+\.\d+   END f2`)
		if !assert.True(t, re.MatchString(buf.String()), "Markers should work") {
			t.Logf("Expected '%v'", re)
			t.Logf("Actual   '%v'", buf.String())
			return
		}

		// TODO: check for error and timestamp
	} else {
		if !assert.Equal(t, "", buf.String(), "Markers should work") {
			return
		}
	}
}
