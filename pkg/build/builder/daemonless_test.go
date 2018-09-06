package builder

import (
	"flag"
	"os"
	"testing"

	"github.com/containers/storage/pkg/reexec"
	"github.com/sirupsen/logrus"
)

func TestMain(m *testing.M) {
	debug := false
	flag.StringVar(&DaemonlessStoreOptions.GraphRoot, "root", DaemonlessStoreOptions.GraphRoot, "storage root dir")
	flag.StringVar(&DaemonlessStoreOptions.RunRoot, "runroot", DaemonlessStoreOptions.RunRoot, "storage state dir")
	flag.StringVar(&DaemonlessStoreOptions.GraphDriverName, "storage-driver", DaemonlessStoreOptions.GraphDriverName, "storage driver")
	flag.BoolVar(&debug, "debug", false, "turn on debug logging")
	flag.Parse()
	if reexec.Init() {
		return
	}
	logrus.SetLevel(logrus.ErrorLevel)
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	ret := m.Run()
	os.Exit(ret)
}
