package main

import (
	"fmt"
	"os"
	"path/filepath"

	tap "github.com/mndrix/tap-go"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/specerror"
	"github.com/opencontainers/runtime-tools/validation/util"
	uuid "github.com/satori/go.uuid"
)

func main() {
	t := tap.New()
	t.Header(0)

	bundleDir, err := util.PrepareBundle()
	if err != nil {
		return
	}
	defer os.RemoveAll(bundleDir)

	g, err := util.GetDefaultGenerator()
	if err != nil {
		util.Fatal(err)
	}
	prestart := rspec.Hook{
		Path: filepath.Join(bundleDir, g.Spec().Root.Path, "/bin/false"),
		Args: []string{"false"},
	}
	g.AddPreStartHook(prestart)
	g.SetProcessArgs([]string{"sh", "-c", fmt.Sprintf("touch %s", "/output")})
	containerID := uuid.NewV4().String()

	config := util.LifecycleConfig{
		Config:    g,
		BundleDir: bundleDir,
		Actions:   util.LifecycleActionCreate | util.LifecycleActionStart,
		PreCreate: func(r *util.Runtime) error {
			r.SetID(containerID)
			return nil
		},
	}

	runErr := util.RuntimeLifecycleValidate(config)
	_, outputErr := os.Stat(filepath.Join(bundleDir, g.Spec().Root.Path, "output"))

	// query the state
	r, _ := util.NewRuntime(util.RuntimeCommand, "")
	r.SetID(containerID)
	_, stateErr := r.State()
	if stateErr != nil {
		// In case a container is created, delete it
		r.Delete()
	}

	// if runErr is nil, it means the runtime does not generate an error
	// if outputErr is nil, it means the runtime calls the Process anyway
	// if stateErr is nil, it means it does not continue lifecycle at step 9
	if runErr == nil || outputErr == nil || stateErr == nil {
		err = specerror.NewError(specerror.PrestartHookFailGenError, fmt.Errorf("if any prestart hook fails, the runtime MUST generate an error, stop the container, and continue the lifecycle at step 9"), rspec.Version)
		diagnostic := map[string]string{
			"error": err.Error(),
		}
		t.YAML(diagnostic)
	}

	t.AutoPlan()
}
