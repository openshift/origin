package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tap "github.com/mndrix/tap-go"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/specerror"
	"github.com/opencontainers/runtime-tools/validation/util"
	uuid "github.com/satori/go.uuid"
)

func main() {
	t := tap.New()
	t.Header(0)

	var output string
	config := util.LifecycleConfig{
		Actions: util.LifecycleActionCreate | util.LifecycleActionStart | util.LifecycleActionDelete,
		PreCreate: func(r *util.Runtime) error {
			r.SetID(uuid.NewV4().String())
			g, err := util.GetDefaultGenerator()
			if err != nil {
				util.Fatal(err)
			}
			output = filepath.Join(r.BundleDir, g.Spec().Root.Path, "output")
			err = g.AddPostStartHook(rspec.Hook{
				Path: filepath.Join(r.BundleDir, g.Spec().Root.Path, "/bin/sh"),
				Args: []string{
					"sh", "-c", fmt.Sprintf("echo 'post-start called' >> %s", output),
				},
			})
			if err != nil {
				return err
			}
			g.SetProcessArgs([]string{"sh", "-c", fmt.Sprintf("echo 'process called' >> %s", "/output")})
			return r.SetConfig(g)
		},
		PostCreate: func(r *util.Runtime) error {
			outputData, err := ioutil.ReadFile(output)
			if err == nil {
				if strings.Contains(string(outputData), "post-start called") {
					return specerror.NewError(specerror.PoststartTiming, fmt.Errorf("The post-start hooks MUST be called before the `start` operation returns"), rspec.Version)
				} else if strings.Contains(string(outputData), "process called") {
					return specerror.NewError(specerror.ProcNotRunAtResRequest, fmt.Errorf("The user-specified program (from process) MUST NOT be run at this time"), rspec.Version)
				}
				return fmt.Errorf("File %v should not exist", output)
			}
			return nil
		},
		PreDelete: func(r *util.Runtime) error {
			util.WaitingForStatus(*r, util.LifecycleStatusStopped, time.Second*10, time.Second)
			outputData, err := ioutil.ReadFile(output)
			if err != nil {
				return fmt.Errorf("%v\n%v", specerror.NewError(specerror.PoststartHooksInvoke, fmt.Errorf("The poststart hooks MUST be invoked by the runtime"), rspec.Version), specerror.NewError(specerror.ProcImplement, fmt.Errorf("The runtime MUST run the user-specified program, as specified by `process`"), rspec.Version))
			}
			switch string(outputData) {
			case "post-start called\n":
				return specerror.NewError(specerror.ProcImplement, fmt.Errorf("The runtime MUST run the user-specified program, as specified by `process`"), rspec.Version)
			case "process called\n":
				fmt.Fprintln(os.Stderr, "WARNING: The poststart hook invoke fails")
				return nil
			case "post-start called\nprocess called\n":
				return specerror.NewError(specerror.PoststartTiming, fmt.Errorf("The post-start hooks MUST be called after the user-specified process is executed"), rspec.Version)
			case "process called\npost-start called\n":
				return nil
			default:
				return fmt.Errorf("Unsupported output information: %v", string(outputData))
			}
		},
	}

	err := util.RuntimeLifecycleValidate(config)
	if err != nil {
		diagnostic := map[string]string{
			"error": err.Error(),
		}
		if e, ok := err.(*exec.ExitError); ok {
			if len(e.Stderr) > 0 {
				diagnostic["stderr"] = string(e.Stderr)
			}
		}
		t.YAML(diagnostic)
	}

	t.AutoPlan()
}
