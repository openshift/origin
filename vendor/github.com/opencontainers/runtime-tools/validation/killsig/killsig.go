package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mndrix/tap-go"
	rspecs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/specerror"
	"github.com/opencontainers/runtime-tools/validation/util"
	uuid "github.com/satori/go.uuid"
)

var signals = []string{
	"TERM",
	"USR1",
	"USR2",
}

func main() {
	t := tap.New()
	t.Header(0)
	bundleDir, err := util.PrepareBundle()
	if err != nil {
		util.Fatal(err)
	}
	defer os.RemoveAll(bundleDir)

	containerID := uuid.NewV4().String()
	sigConfig, err := util.GetDefaultGenerator()
	if err != nil {
		os.RemoveAll(bundleDir)
		util.Fatal(err)
	}
	rootDir := filepath.Join(bundleDir, sigConfig.Spec().Root.Path)
	for _, signal := range signals {
		sigConfig.SetProcessArgs([]string{"sh", "-c", fmt.Sprintf("trap 'touch /%s' %s; sleep 10 & wait $!", signal, signal)})
		config := util.LifecycleConfig{
			Config:    sigConfig,
			BundleDir: bundleDir,
			Actions:   util.LifecycleActionCreate | util.LifecycleActionStart | util.LifecycleActionDelete,
			PreCreate: func(r *util.Runtime) error {
				r.SetID(containerID)
				return nil
			},
			PreDelete: func(r *util.Runtime) error {
				util.WaitingForStatus(*r, util.LifecycleStatusRunning, time.Second*5, time.Second*1)
				err = r.Kill(signal)
				// wait before the container been deleted
				util.WaitingForStatus(*r, util.LifecycleStatusStopped, time.Second*5, time.Second*1)
				return err
			},
		}
		err = util.RuntimeLifecycleValidate(config)
		if err != nil {
			util.SpecErrorOK(t, false, specerror.NewError(specerror.KillSignalImplement, fmt.Errorf("`kill` operation MUST send the specified signal to the container process"), rspecs.Version), err)
		} else {
			_, err = os.Stat(filepath.Join(rootDir, signal))
			util.SpecErrorOK(t, err == nil, specerror.NewError(specerror.KillSignalImplement, fmt.Errorf("`kill` operation MUST send the specified signal to the container process"), rspecs.Version), err)
		}
	}
	t.AutoPlan()
}
