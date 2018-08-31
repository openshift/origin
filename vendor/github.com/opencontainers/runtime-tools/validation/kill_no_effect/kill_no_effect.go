package main

import (
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/mndrix/tap-go"
	rspecs "github.com/opencontainers/runtime-spec/specs-go"
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

	targetErr := specerror.NewError(specerror.KillNonCreateRunHaveNoEffect, fmt.Errorf("attempting to send a signal to a container that is neither `created` nor `running` MUST have no effect on the container"), rspecs.Version)
	containerID := uuid.NewV4().String()
	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	g.SetProcessArgs([]string{"true"})

	config := util.LifecycleConfig{
		Config:    g,
		BundleDir: bundleDir,
		Actions:   util.LifecycleActionCreate | util.LifecycleActionStart | util.LifecycleActionDelete,
		PreCreate: func(r *util.Runtime) error {
			r.SetID(containerID)
			return nil
		},
		PreDelete: func(r *util.Runtime) error {
			err := util.WaitingForStatus(*r, util.LifecycleStatusStopped, time.Second*5, time.Second*1)
			if err != nil {
				return err
			}
			currentState, err := r.State()
			if err != nil {
				return err
			}
			r.Kill("KILL")
			newState, err := r.State()
			if err != nil || !reflect.DeepEqual(newState, currentState) {
				return targetErr
			}
			return nil
		},
	}
	err = util.RuntimeLifecycleValidate(config)
	if err != nil && err != targetErr {
		t.Fail(err.Error())
	} else {
		util.SpecErrorOK(t, err == nil, targetErr, nil)
	}
	t.AutoPlan()
}
