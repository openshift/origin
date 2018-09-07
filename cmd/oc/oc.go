package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/util/logs"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	"github.com/openshift/api/apps"
	"github.com/openshift/api/authorization"
	"github.com/openshift/api/build"
	"github.com/openshift/api/image"
	"github.com/openshift/api/network"
	"github.com/openshift/api/oauth"
	"github.com/openshift/api/operator"
	"github.com/openshift/api/project"
	"github.com/openshift/api/quota"
	"github.com/openshift/api/route"
	"github.com/openshift/api/security"
	"github.com/openshift/api/template"
	"github.com/openshift/api/user"
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
	// We can't use the "normal" scheme because apply will use that to build stategic merge patches on CustomResources
	utilruntime.Must(apps.Install(scheme.Scheme))
	utilruntime.Must(authorization.Install(scheme.Scheme))
	utilruntime.Must(build.Install(scheme.Scheme))
	utilruntime.Must(image.Install(scheme.Scheme))
	utilruntime.Must(network.Install(scheme.Scheme))
	utilruntime.Must(oauth.Install(scheme.Scheme))
	utilruntime.Must(operator.Install(scheme.Scheme))
	utilruntime.Must(project.Install(scheme.Scheme))
	utilruntime.Must(quota.Install(scheme.Scheme))
	utilruntime.Must(route.Install(scheme.Scheme))
	utilruntime.Must(security.Install(scheme.Scheme))
	utilruntime.Must(template.Install(scheme.Scheme))
	utilruntime.Must(user.Install(scheme.Scheme))
	legacy.InstallExternalLegacyAll(scheme.Scheme)

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
