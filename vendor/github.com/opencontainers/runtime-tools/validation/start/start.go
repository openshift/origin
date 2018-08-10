package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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
	defer t.AutoPlan()

	bundleDir, err := util.PrepareBundle()
	if err != nil {
		util.Fatal(err)
	}
	defer os.RemoveAll(bundleDir)

	containerID := uuid.NewV4().String()

	r, err := util.NewRuntime(util.RuntimeCommand, bundleDir)
	if err != nil {
		util.Fatal(err)
	}
	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	g.SetProcessArgs([]string{"sh", "-c", fmt.Sprintf("echo 'process called' >> %s", "/output")})
	err = r.SetConfig(g)
	if err != nil {
		util.Fatal(err)
	}
	output := filepath.Join(bundleDir, g.Spec().Root.Path, "output")

	// start without id
	err = r.Start()
	util.SpecErrorOK(t, err != nil, specerror.NewError(specerror.StartWithoutIDGenError, fmt.Errorf("start` operation MUST generate an error if it is not provided the container ID"), rspecs.Version), err)

	// set id for the remaining tests
	r.SetID(containerID)

	// start a not `created` container - case one: non-exist container
	err = r.Start()
	util.SpecErrorOK(t, err != nil, specerror.NewError(specerror.StartNotCreatedGenError, fmt.Errorf("attempting to `start` a container that is not `created` MUST generate an error"), rspecs.Version), err)

	err = r.Create()
	if err != nil {
		t.Fail(err.Error())
		return
	}
	_, err = os.Stat(output)
	// check the existence of the output file
	util.SpecErrorOK(t, err != nil && os.IsNotExist(err), specerror.NewError(specerror.ProcArgsApplyUntilStart, fmt.Errorf("`process.args` MUST NOT be applied until triggered by the start operation"), rspecs.Version), err)

	// start a `created` container
	err = r.Start()
	if err != nil {
		util.SpecErrorOK(t, false, specerror.NewError(specerror.StartProcImplement, fmt.Errorf("`start` operation MUST run the user-specified program as specified by `process`"), rspecs.Version), err)
	} else {
		err = util.WaitingForStatus(r, util.LifecycleStatusStopped, time.Second*10, time.Second*1)
		if err != nil {
			t.Fail(err.Error())
			return
		}
		outputData, outputErr := ioutil.ReadFile(output)
		// check the output
		util.SpecErrorOK(t, outputErr == nil && string(outputData) == "process called\n", specerror.NewError(specerror.StartProcImplement, fmt.Errorf("`start` operation MUST run the user-specified program as specified by `process`"), rspecs.Version), outputErr)
	}

	// start a not `created` container - case two: exist and `stopped`
	err = r.Start()
	// must generate an error
	util.SpecErrorOK(t, err != nil, specerror.NewError(specerror.StartNotCreatedGenError, fmt.Errorf("attempting to `start` a container that is not `created` MUST generate an error"), rspecs.Version), err)

	err = util.WaitingForStatus(r, util.LifecycleStatusStopped, time.Second*10, time.Second*1)
	if err != nil {
		t.Fail(err.Error())
		return
	}

	outputData, outputErr := ioutil.ReadFile(output)
	// must have no effect, it will not be something like 'process called\nprocess called\n'
	util.SpecErrorOK(t, outputErr == nil && string(outputData) == "process called\n", specerror.NewError(specerror.StartNotCreatedHaveNoEffect, fmt.Errorf("attempting to `start` a container that is not `created` MUST have no effect on the container"), rspecs.Version), outputErr)

	err = r.Delete()
	if err != nil {
		t.Fail(err.Error())
		return
	}

	g.Spec().Process = nil
	err = r.SetConfig(g)
	if err != nil {
		util.Fatal(err)
	}
	err = r.Create()
	if err != nil {
		t.Fail(err.Error())
		return
	}

	err = r.Start()
	util.SpecErrorOK(t, err == nil, specerror.NewError(specerror.StartWithProcUnsetGenError, fmt.Errorf("`start` operation MUST generate an error if `process` was not set"), rspecs.Version), err)
	err = util.WaitingForStatus(r, util.LifecycleStatusStopped, time.Second*10, time.Second*1)
	if err == nil {
		err = r.Delete()
	}
	if err != nil {
		t.Fail(err.Error())
	}
}
