package main

import (
	"math/rand"
	"os"
	"runtime"
	"time"

	"k8s.io/apiserver/pkg/util/logs"

	"github.com/openshift/origin/pkg/cmd/util/serviceability"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics"
	"github.com/spf13/cobra"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/apis/autoscaling/install"
	_ "k8s.io/kubernetes/pkg/apis/batch/install"
	_ "k8s.io/kubernetes/pkg/apis/core/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()
	defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"))()
	defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()

	rand.Seed(time.Now().UTC().UnixNano())
	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	cmd := &cobra.Command{
		Use:   "openshift-diagnostics",
		Short: "Diagnose OpenShift clusters",
	}
	cmd.AddCommand(
		diagnostics.NewCommandPodDiagnostics("diagnostic-pod", os.Stdout),
		diagnostics.NewCommandNetworkPodDiagnostics("network-diagnostic-pod", os.Stdout),
	)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
