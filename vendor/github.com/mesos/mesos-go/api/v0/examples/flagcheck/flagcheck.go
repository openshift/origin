package main

import (
	"flag"
	"fmt"
	"os"

	// Import all non-testing packages to verify that flags are not added
	// to the command line.

	_ "github.com/mesos/mesos-go/api/v0/auth"
	_ "github.com/mesos/mesos-go/api/v0/auth/callback"
	_ "github.com/mesos/mesos-go/api/v0/auth/sasl"
	_ "github.com/mesos/mesos-go/api/v0/auth/sasl/mech"
	_ "github.com/mesos/mesos-go/api/v0/auth/sasl/mech/crammd5"
	_ "github.com/mesos/mesos-go/api/v0/detector"
	_ "github.com/mesos/mesos-go/api/v0/detector/zoo"
	_ "github.com/mesos/mesos-go/api/v0/executor"
	_ "github.com/mesos/mesos-go/api/v0/healthchecker"
	_ "github.com/mesos/mesos-go/api/v0/mesosproto"
	_ "github.com/mesos/mesos-go/api/v0/mesosproto/scheduler"
	_ "github.com/mesos/mesos-go/api/v0/mesosutil"
	_ "github.com/mesos/mesos-go/api/v0/mesosutil/process"
	_ "github.com/mesos/mesos-go/api/v0/messenger"
	_ "github.com/mesos/mesos-go/api/v0/messenger/sessionid"
	_ "github.com/mesos/mesos-go/api/v0/scheduler"
	_ "github.com/mesos/mesos-go/api/v0/upid"
)

// Flags which are accepted from other packages.
var allowedFlags = []string{
	// Flags added from the glog package
	"logtostderr",
	"alsologtostderr",
	"v",
	"stderrthreshold",
	"vmodule",
	"log_backtrace_at",
	"log_dir",
}

func main() {
	expected := map[string]struct{}{}
	for _, f := range allowedFlags {
		expected[f] = struct{}{}
	}

	hasLeak := false
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		if _, ok := expected[f.Name]; !ok {
			fmt.Fprintf(os.Stderr, "Leaking flag %q: %q\n", f.Name, f.Usage)
			hasLeak = true
		}
	})

	if hasLeak {
		os.Exit(1)
	}
}
