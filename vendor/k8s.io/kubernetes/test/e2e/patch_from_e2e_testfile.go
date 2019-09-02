package e2e

import (
	"fmt"
	"os"

	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/testfiles"
	"k8s.io/kubernetes/test/e2e/framework/viperconfig"
	"k8s.io/kubernetes/test/e2e/generated"
	"k8s.io/kubernetes/test/utils/image"
)

// this function matches the init block from e2e_test.go
func ViperizeFlags(viperConfig string) {
	// Register framework flags, then handle flags and Viper config.
	framework.HandleFlags()
	if err := viperconfig.ViperizeFlags(viperConfig, "e2e"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if framework.TestContext.ListImages {
		for _, v := range image.GetImageConfigs() {
			fmt.Println(v.GetE2EImage())
		}
		os.Exit(0)
	}

	framework.AfterReadingAllFlags(&framework.TestContext)

	// this came from the init block, but it breaks on openshift.  Not really sure why.
	// TODO: Deprecating repo-root over time... instead just use gobindata_util.go , see #23987.
	// Right now it is still needed, for example by
	// test/e2e/framework/ingress/ingress_utils.go
	// for providing the optional secret.yaml file and by
	// test/e2e/framework/util.go for cluster/log-dump.
	//if framework.TestContext.RepoRoot != "" {
	//	testfiles.AddFileSource(testfiles.RootFileSource{Root: framework.TestContext.RepoRoot})
	//}

	// Enable bindata file lookup as fallback.
	testfiles.AddFileSource(testfiles.BindataFileSource{
		Asset:      generated.Asset,
		AssetNames: generated.AssetNames,
	})

}

var localViperConfig = ""

// we appear to set ours via env-var, not flag
func GetViperConfig() string {
	return localViperConfig
}

func SetViperConfig(val string) {
	localViperConfig = val
}
