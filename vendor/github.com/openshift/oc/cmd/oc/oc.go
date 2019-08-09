package main

import (
	goflag "flag"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/pflag"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	"github.com/openshift/api/apps"
	"github.com/openshift/api/authorization"
	"github.com/openshift/api/build"
	"github.com/openshift/api/image"
	"github.com/openshift/api/network"
	"github.com/openshift/api/oauth"
	"github.com/openshift/api/project"
	"github.com/openshift/api/quota"
	"github.com/openshift/api/route"
	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/api/template"
	"github.com/openshift/api/user"
	"github.com/openshift/library-go/pkg/serviceability"

	"github.com/openshift/oc/pkg/cli"
	"github.com/openshift/oc/pkg/helpers/legacy"
	"github.com/openshift/oc/pkg/version"
)

func injectLoglevelFlag(flags *pflag.FlagSet) {
	from := goflag.CommandLine
	if flag := from.Lookup("v"); flag != nil {
		level := flag.Value.(*klog.Level)
		levelPtr := (*int32)(level)
		flags.Int32Var(levelPtr, "loglevel", 0, "Set the level of log output (0-10)")
		if flags.Lookup("v") == nil {
			flags.Int32Var(levelPtr, "v", 0, "Set the level of log output (0-10)")
		}
	}
}

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()
	defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"), version.Get())()
	defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()

	rand.Seed(time.Now().UTC().UnixNano())
	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	pflag.CommandLine.SetNormalizeFunc(utilflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	injectLoglevelFlag(pflag.CommandLine)

	// the kubectl scheme expects to have all the recognizable external types it needs to consume.  Install those here.
	// We can't use the "normal" scheme because apply will use that to build stategic merge patches on CustomResources
	utilruntime.Must(apps.Install(scheme.Scheme))
	utilruntime.Must(authorization.Install(scheme.Scheme))
	utilruntime.Must(build.Install(scheme.Scheme))
	utilruntime.Must(image.Install(scheme.Scheme))
	utilruntime.Must(network.Install(scheme.Scheme))
	utilruntime.Must(oauth.Install(scheme.Scheme))
	utilruntime.Must(project.Install(scheme.Scheme))
	utilruntime.Must(quota.Install(scheme.Scheme))
	utilruntime.Must(route.Install(scheme.Scheme))
	utilruntime.Must(installNonCRDSecurity(scheme.Scheme))
	utilruntime.Must(template.Install(scheme.Scheme))
	utilruntime.Must(user.Install(scheme.Scheme))
	legacy.InstallExternalLegacyAll(scheme.Scheme)

	// the legacyscheme is used in kubectl convert and get, so we need to
	// explicitly install all types there too
	utilruntime.Must(apps.Install(legacyscheme.Scheme))
	utilruntime.Must(authorization.Install(legacyscheme.Scheme))
	utilruntime.Must(build.Install(legacyscheme.Scheme))
	utilruntime.Must(image.Install(legacyscheme.Scheme))
	utilruntime.Must(network.Install(legacyscheme.Scheme))
	utilruntime.Must(oauth.Install(legacyscheme.Scheme))
	utilruntime.Must(project.Install(legacyscheme.Scheme))
	utilruntime.Must(quota.Install(legacyscheme.Scheme))
	utilruntime.Must(route.Install(legacyscheme.Scheme))
	utilruntime.Must(installNonCRDSecurity(legacyscheme.Scheme))
	utilruntime.Must(template.Install(legacyscheme.Scheme))
	utilruntime.Must(user.Install(legacyscheme.Scheme))
	legacy.InstallExternalLegacyAll(legacyscheme.Scheme)

	basename := filepath.Base(os.Args[0])
	command := cli.CommandFor(basename)
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}

func installNonCRDSecurity(scheme *apimachineryruntime.Scheme) error {
	scheme.AddKnownTypes(securityv1.GroupVersion,
		&securityv1.PodSecurityPolicySubjectReview{},
		&securityv1.PodSecurityPolicySelfSubjectReview{},
		&securityv1.PodSecurityPolicyReview{},
		&securityv1.RangeAllocation{},
		&securityv1.RangeAllocationList{},
	)
	if err := corev1.AddToScheme(scheme); err != nil {
		return err
	}
	metav1.AddToGroupVersion(scheme, securityv1.GroupVersion)
	return nil
}
