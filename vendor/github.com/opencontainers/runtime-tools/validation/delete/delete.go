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
	testRuntime, _ := util.NewRuntime(util.RuntimeCommand, bundleDir)
	cases := []struct {
		config      *generate.Generator
		id          string
		action      util.LifecycleAction
		errExpected bool
		// set true to check whether 'delete' takes effect and just check if 'delete' DOES NOT take effect
		effectCheck bool
		err         error
	}{
		// Note: the nil config test case should run first since we are re-using the bundle
		// delete without id
		{nil, "", util.LifecycleActionNone, false, false, specerror.NewError(specerror.DeleteWithoutIDGenError, fmt.Errorf("`delete` operation MUST generate an error if it is not provided the container ID"), rspecs.Version)},
		// delete a created container
		{stoppedConfig, containerID, util.LifecycleActionCreate, false, true, specerror.NewError(specerror.DeleteNonStopGenError, fmt.Errorf("attempting to `delete` a container that is not `stopped` MUST generate an error"), rspecs.Version)},
		// delete a running container
		{runningConfig, containerID, util.LifecycleActionCreate | util.LifecycleActionStart, false, true, specerror.NewError(specerror.DeleteNonStopGenError, fmt.Errorf("attempting to `delete` a container that is not `stopped` MUST generate an error"), rspecs.Version)},
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
		}
		util.RuntimeLifecycleValidate(config)
		// waiting the 'stoppedConfig' testcase to stop
		// the 'runningConfig' testcase sleeps 30 seconds, so 10 seconds are enough for this case
		testRuntime.SetID(c.id)
		util.WaitingForStatus(testRuntime, util.LifecycleStatusCreated|util.LifecycleStatusStopped, time.Second*10, time.Second*1)
		deletedErr := testRuntime.Delete()
		util.SpecErrorOK(t, (deletedErr == nil) == c.errExpected, c.err, deletedErr)

		if c.effectCheck {
			// waiting for the error of State, just in case the delete operation takes time
			util.WaitingForStatus(testRuntime, util.LifecycleActionNone, time.Second*10, time.Second*1)
			_, err = testRuntime.State()
			// err == nil means the 'delete' operation does NOT take effect
			util.SpecErrorOK(t, err == nil, specerror.NewError(specerror.DeleteNonStopHaveNoEffect, fmt.Errorf("attempting to `delete` a container that is not `stopped` MUST have no effect on the container"), rspecs.Version), err)
		}

		// created and but deleted
		if (c.action&util.LifecycleActionCreate != 0) && (deletedErr != nil) {
			testRuntime.Kill("KILL")
			// waiting for the container to be killed, just in case the kill operation takes time
			util.WaitingForStatus(testRuntime, util.LifecycleStatusStopped, time.Second*10, time.Second*1)
			err = testRuntime.Delete()
			if err != nil {
				t.Fail(err.Error())
			}
		}
	}

	t.AutoPlan()
}
