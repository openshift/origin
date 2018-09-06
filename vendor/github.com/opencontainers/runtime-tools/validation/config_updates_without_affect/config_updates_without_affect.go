package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tap "github.com/mndrix/tap-go"
	"github.com/mrunalp/fileutils"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/runtime-tools/specerror"
	"github.com/opencontainers/runtime-tools/validation/util"
	uuid "github.com/satori/go.uuid"
)

func main() {
	t := tap.New()
	t.Header(0)

	bundleDir, err := util.PrepareBundle()
	if err != nil {
		util.Fatal(err)
	}
	defer os.RemoveAll(bundleDir)

	r, err := util.NewRuntime(util.RuntimeCommand, bundleDir)
	if err != nil {
		util.Fatal(err)
	}

	testPath := filepath.Join(bundleDir, "test.json")
	r.SetID(uuid.NewV4().String())
	// generate a config has all the testing properties
	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	g.SetProcessArgs([]string{"/runtimetest", "--path=/test.json"})
	g.AddLinuxMaskedPaths("/proc/kcore")
	g.AddLinuxReadonlyPaths("/proc/fs")
	g.AddLinuxSysctl("net.ipv4.ip_forward", "1")
	g.SetProcessOOMScoreAdj(100)
	g.AddProcessRlimits("RLIMIT_NOFILE", 1024, 1024)
	g.SetLinuxRootPropagation("shared")

	err = r.SetConfig(g)
	if err != nil {
		util.Fatal(err)
	}

	err = g.SaveToFile(testPath, generate.ExportOptions{})
	if err != nil {
		util.Fatal(err)
	}

	err = fileutils.CopyFile("runtimetest", filepath.Join(r.BundleDir, "runtimetest"))
	if err != nil {
		util.Fatal(err)
	}

	err = r.Create()
	if err != nil {
		util.Fatal(err)
	}

	spec := &rspec.Spec{
		Version: "1.0.0",
	}
	g.SetSpec(spec)
	err = r.SetConfig(g)
	if err != nil {
		util.Fatal(err)
	}

	err = r.Start()
	util.SpecErrorOK(t, err == nil, specerror.NewError(specerror.ConfigUpdatesWithoutAffect, fmt.Errorf("Any updates to config.json after this step MUST NOT affect the container"), rspec.Version), nil)

	err = util.WaitingForStatus(r, util.LifecycleStatusStopped, time.Second*10, time.Second*1)
	if err == nil {
		err = r.Delete()
	}
	if err != nil {
		t.Fail(err.Error())
	}

	t.AutoPlan()
}
