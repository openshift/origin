package serviceability

import (
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/pkg/profile"
)

// Stop is a function to defer in your main call to provide profile info.
type Stop interface {
	Stop()
}

type stopper struct{}

func (stopper) Stop() {}

// Profile returns an interface to defer for a profile: `defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()` is common.
// Suffixing the mode with `-tmp` will have the profiler write the run to a temporary directory with a unique name, which
// is useful when running the same command multiple times.
func Profile(mode string) Stop {
	path := "."
	if strings.HasSuffix(mode, "-tmp") {
		mode = strings.TrimSuffix(mode, "-tmp")
		path = ""
	}
	var stop Stop
	switch mode {
	case "mem":
		stop = profileOnExit(profile.Start(profile.MemProfile, profile.ProfilePath(path), profile.NoShutdownHook, profile.Quiet))
	case "cpu":
		stop = profileOnExit(profile.Start(profile.CPUProfile, profile.ProfilePath(path), profile.NoShutdownHook, profile.Quiet))
	case "block":
		stop = profileOnExit(profile.Start(profile.BlockProfile, profile.ProfilePath(path), profile.NoShutdownHook, profile.Quiet))
	case "mutex":
		stop = profileOnExit(profile.Start(profile.MutexProfile, profile.ProfilePath(path), profile.NoShutdownHook, profile.Quiet))
	case "trace":
		stop = profileOnExit(profile.Start(profile.TraceProfile, profile.ProfilePath(path), profile.NoShutdownHook, profile.Quiet))
	default:
		stop = stopper{}
	}
	return stop
}

func profileOnExit(s Stop) Stop {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		// Programs with more sophisticated signal handling
		// should ensure the Stop() function returned from
		// Start() is called during shutdown.
		// See http://godoc.org/github.com/pkg/profile
		s.Stop()

		os.Exit(1)
	}()
	return s
}
