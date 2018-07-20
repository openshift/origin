package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"k8s.io/apiserver/pkg/util/logs"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	"github.com/openshift/api"
	"github.com/openshift/api/authorization"
	"github.com/openshift/api/quota"
	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/openshift/origin/pkg/api/install"
	"github.com/openshift/origin/pkg/api/legacy"
	"github.com/openshift/origin/pkg/oc/cli"
	"github.com/openshift/origin/pkg/version"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()
	defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"), version.Get())()
	defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()

	rand.Seed(time.Now().UTC().UnixNano())
	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	// the kubectl scheme expects to have all the recognizable external types it needs to consume.  Install those here.
	api.Install(scheme.Scheme)
	legacy.InstallExternalLegacyAll(scheme.Scheme)
	// TODO fix up the install for the "all types"
	authorization.Install(scheme.Scheme)
	quota.Install(scheme.Scheme)

	// the legacyscheme is used in kubectl and expects to have the internal types registered.  Explicitly wire our types here.
	// this does
	install.InstallInternalOpenShift(legacyscheme.Scheme)
	legacy.InstallInternalLegacyAll(scheme.Scheme)

	basename := filepath.Base(os.Args[0])
	command := cli.CommandFor(basename)
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
