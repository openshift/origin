package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mndrix/tap-go"
	rspecs "github.com/opencontainers/runtime-spec/specs-go"
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

	stoppedConfig, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	stoppedConfig.SetProcessArgs([]string{"true"})
	runningConfig, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	runningConfig.SetProcessArgs([]string{"sleep", "30"})
	containerID := uuid.NewV4().String()

	cases := []struct {
		config      *generate.Generator
		id          string
		action      util.LifecycleAction
		errExpected bool
		err         error
	}{
		// Note: the nil config test case should run first since we are re-using the bundle
		// kill without id
		{nil, "", util.LifecycleActionNone, false, specerror.NewError(specerror.KillWithoutIDGenError, fmt.Errorf("`kill` operation MUST generate an error if it is not provided the container ID"), rspecs.Version)},
		// kill a non exist container
		{nil, containerID, util.LifecycleActionNone, false, specerror.NewError(specerror.KillNonCreateRunGenError, fmt.Errorf("attempting to send a signal to a container that is neither `created` nor `running` MUST generate an error"), rspecs.Version)},
		// kill a created
		{stoppedConfig, containerID, util.LifecycleActionCreate | util.LifecycleActionDelete, true, specerror.NewError(specerror.KillSignalImplement, fmt.Errorf("`kill` operation MUST send the specified signal to the container process"), rspecs.Version)},
		// kill a stopped
		{stoppedConfig, containerID, util.LifecycleActionCreate | util.LifecycleActionStart | util.LifecycleActionDelete, false, specerror.NewError(specerror.KillNonCreateRunGenError, fmt.Errorf("attempting to send a signal to a container that is neither `created` nor `running` MUST generate an error"), rspecs.Version)},
		// kill a running
		{runningConfig, containerID, util.LifecycleActionCreate | util.LifecycleActionStart | util.LifecycleActionDelete, true, specerror.NewError(specerror.KillSignalImplement, fmt.Errorf("`kill` operation MUST send the specified signal to the container process"), rspecs.Version)},
	}

	for _, c := range cases {
		config := util.LifecycleConfig{
			Config:    c.config,
			BundleDir: bundleDir,
			Actions:   c.action,
			PreCreate: func(r *util.Runtime) error {
				r.SetID(c.id)
				return nil
			},
			PreDelete: func(r *util.Runtime) error {
				// waiting the 'stoppedConfig' testcase to stop
				// the 'runningConfig' testcase sleeps 30 seconds, so 10 seconds are enough for this case
				util.WaitingForStatus(*r, util.LifecycleStatusCreated|util.LifecycleStatusStopped, time.Second*10, time.Second*1)
				// KILL MUST be supported and KILL cannot be trapped
				err = r.Kill("KILL")
				util.WaitingForStatus(*r, util.LifecycleStatusStopped, time.Second*10, time.Second*1)
				return err
			},
		}
		err = util.RuntimeLifecycleValidate(config)
		util.SpecErrorOK(t, (err == nil) == c.errExpected, c.err, err)
	}

	t.AutoPlan()
}
