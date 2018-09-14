package util

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mndrix/tap-go"
	"github.com/mrunalp/fileutils"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/runtime-tools/specerror"
	"github.com/satori/go.uuid"
)

var (
	// RuntimeCommand is the default runtime command.
	RuntimeCommand = "runc"
)

// LifecycleAction defines the phases will be called.
type LifecycleAction int

const (
	// LifecycleActionNone does nothing
	LifecycleActionNone = 0
	// LifecycleActionCreate creates a container
	LifecycleActionCreate = 1 << iota
	// LifecycleActionStart starts a container
	LifecycleActionStart
	// LifecycleActionDelete deletes a container
	LifecycleActionDelete
)

// LifecycleStatus follows https://github.com/opencontainers/runtime-spec/blob/master/runtime.md#state
type LifecycleStatus int

const (
	// LifecycleStatusCreating "creating"
	LifecycleStatusCreating = 1 << iota
	// LifecycleStatusCreated "created"
	LifecycleStatusCreated
	// LifecycleStatusRunning "running"
	LifecycleStatusRunning
	// LifecycleStatusStopped "stopped"
	LifecycleStatusStopped
)

var lifecycleStatusMap = map[string]LifecycleStatus{
	"creating": LifecycleStatusCreating,
	"created":  LifecycleStatusCreated,
	"running":  LifecycleStatusRunning,
	"stopped":  LifecycleStatusStopped,
}

// LifecycleConfig includes
// 1. Config to set the 'config.json'
// 2. BundleDir to set the bundle directory
// 3. Actions to define the default running lifecycles
// 4. Four phases for user to add his/her own operations
type LifecycleConfig struct {
	Config     *generate.Generator
	BundleDir  string
	Actions    LifecycleAction
	PreCreate  func(runtime *Runtime) error
	PostCreate func(runtime *Runtime) error
	PreDelete  func(runtime *Runtime) error
	PostDelete func(runtime *Runtime) error
}

// PreFunc initializes the test environment after preparing the bundle
// but before creating the container.
type PreFunc func(string) error

// AfterFunc validate container's outside environment after created
type AfterFunc func(config *rspec.Spec, t *tap.T, state *rspec.State) error

func init() {
	runtimeInEnv := os.Getenv("RUNTIME")
	if runtimeInEnv != "" {
		RuntimeCommand = runtimeInEnv
	}
}

// Fatal prints a warning to stderr and exits.
func Fatal(err error) {
	fmt.Fprintf(os.Stderr, "%+v\n", err)
	os.Exit(1)
}

// Skip skips a full TAP suite.
func Skip(message string, diagnostic interface{}) {
	t := tap.New()
	t.Header(1)
	t.Skip(1, message)
	if diagnostic != nil {
		t.YAML(diagnostic)
	}
}

// SpecErrorOK generates TAP output indicating whether a spec code test passed or failed.
func SpecErrorOK(t *tap.T, expected bool, specErr error, detailedErr error) {
	t.Ok(expected, specErr.(*specerror.Error).Err.Err.Error())
	diagnostic := map[string]string{
		"reference": specErr.(*specerror.Error).Err.Reference,
	}

	if detailedErr != nil {
		diagnostic["error"] = detailedErr.Error()
		if e, ok := detailedErr.(*exec.ExitError); ok {
			if len(e.Stderr) > 0 {
				diagnostic["stderr"] = string(e.Stderr)
			}
		}
	}
	t.YAML(diagnostic)
}

// PrepareBundle creates a test bundle in a temporary directory.
func PrepareBundle() (string, error) {
	bundleDir, err := ioutil.TempDir("", "ocitest")
	if err != nil {
		return "", err
	}

	// Untar the root fs
	untarCmd := exec.Command("tar", "-xf", fmt.Sprintf("rootfs-%s.tar.gz", runtime.GOARCH), "-C", bundleDir)
	output, err := untarCmd.CombinedOutput()
	if err != nil {
		os.Stderr.Write(output)
		os.RemoveAll(bundleDir)
		return "", err
	}

	return bundleDir, nil
}

// GetDefaultGenerator creates a default configuration generator.
func GetDefaultGenerator() (*generate.Generator, error) {
	g, err := generate.New(runtime.GOOS)
	if err != nil {
		return nil, err
	}
	g.SetRootPath(".")
	g.SetProcessArgs([]string{"/runtimetest", "--path=/"})
	return &g, err
}

// WaitingForStatus waits an expected runtime status, return error if
// 1. fail to query the status
// 2. timeout
func WaitingForStatus(r Runtime, status LifecycleStatus, retryTimeout time.Duration, pollInterval time.Duration) error {
	for start := time.Now(); time.Since(start) < retryTimeout; time.Sleep(pollInterval) {
		state, err := r.State()
		if err != nil {
			return err
		}
		if v, ok := lifecycleStatusMap[state.Status]; ok {
			if status&v != 0 {
				return nil
			}
		} else {
			// In spec, it says 'Additional values MAY be defined by the runtime'.
			continue
		}
	}

	return errors.New("timeout in waiting for the container status")
}

var runtimeInsideValidateCalled bool

// RuntimeInsideValidate runs runtimetest inside a container.
func RuntimeInsideValidate(g *generate.Generator, t *tap.T, f PreFunc) (err error) {
	bundleDir, err := PrepareBundle()
	if err != nil {
		return err
	}

	if f != nil {
		if err := f(bundleDir); err != nil {
			return err
		}
	}

	r, err := NewRuntime(RuntimeCommand, bundleDir)
	if err != nil {
		os.RemoveAll(bundleDir)
		return err
	}
	defer r.Clean(true, true)
	err = r.SetConfig(g)
	if err != nil {
		return err
	}
	err = fileutils.CopyFile("runtimetest", filepath.Join(r.BundleDir, "runtimetest"))
	if err != nil {
		return err
	}

	r.SetID(uuid.NewV4().String())
	err = r.Create()
	if err != nil {
		os.Stderr.WriteString("failed to create the container\n")
		if e, ok := err.(*exec.ExitError); ok && len(e.Stderr) > 0 {
			os.Stderr.Write(e.Stderr)
		}
		return err
	}

	err = r.Start()
	if err != nil {
		os.Stderr.WriteString("failed to start the container\n")
		if e, ok := err.(*exec.ExitError); ok && len(e.Stderr) > 0 {
			os.Stderr.Write(e.Stderr)
		}
		return err
	}

	err = WaitingForStatus(r, LifecycleStatusStopped, 10*time.Second, 1*time.Second)
	if err != nil {
		return err
	}

	stdout, stderr, err := r.ReadStandardStreams()
	if err != nil {
		if len(stderr) == 0 {
			stderr = stdout
		}
		os.Stderr.WriteString("failed to read standard streams\n")
		os.Stderr.Write(stderr)
		return err
	}

	// Write stdout in the outter TAP
	if t != nil {
		diagnostic := map[string]string{
			"stdout": string(stdout),
			"stderr": string(stderr),
		}
		if err != nil {
			diagnostic["error"] = fmt.Sprintf("%v", err)
		}
		t.YAML(diagnostic)
		t.Ok(err == nil && !strings.Contains(string(stdout), "not ok"), g.Config.Annotations["TestName"])
	} else {
		if runtimeInsideValidateCalled {
			Fatal(errors.New("RuntimeInsideValidate called several times in the same test without passing TAP"))
		}
		runtimeInsideValidateCalled = true
		os.Stdout.Write(stdout)
	}
	return nil
}

// RuntimeOutsideValidate validate runtime outside a container.
func RuntimeOutsideValidate(g *generate.Generator, t *tap.T, f AfterFunc) error {
	bundleDir, err := PrepareBundle()
	if err != nil {
		return err
	}

	r, err := NewRuntime(RuntimeCommand, bundleDir)
	if err != nil {
		os.RemoveAll(bundleDir)
		return err
	}
	defer r.Clean(true, true)
	err = r.SetConfig(g)
	if err != nil {
		return err
	}
	err = fileutils.CopyFile("runtimetest", filepath.Join(r.BundleDir, "runtimetest"))
	if err != nil {
		return err
	}

	r.SetID(uuid.NewV4().String())
	err = r.Create()
	if err != nil {
		os.Stderr.WriteString("failed to create the container\n")
		if e, ok := err.(*exec.ExitError); ok && len(e.Stderr) > 0 {
			os.Stderr.Write(e.Stderr)
		}
		return err
	}

	if f != nil {
		state, err := r.State()
		if err != nil {
			return err
		}
		if err := f(g.Spec(), t, &state); err != nil {
			return err
		}
	}
	return nil
}

// RuntimeLifecycleValidate validates runtime lifecycle.
func RuntimeLifecycleValidate(config LifecycleConfig) error {
	var bundleDir string
	var err error

	if config.BundleDir == "" {
		bundleDir, err = PrepareBundle()
		if err != nil {
			return err
		}
		defer os.RemoveAll(bundleDir)
	} else {
		bundleDir = config.BundleDir
	}

	r, err := NewRuntime(RuntimeCommand, bundleDir)
	if err != nil {
		return err
	}

	if config.Config != nil {
		if err := r.SetConfig(config.Config); err != nil {
			return err
		}
	}

	if config.PreCreate != nil {
		if err := config.PreCreate(&r); err != nil {
			return err
		}
	}

	if config.Actions&LifecycleActionCreate != 0 {
		err := r.Create()
		if err != nil {
			os.Stderr.WriteString("failed to create the container\n")
			if e, ok := err.(*exec.ExitError); ok && len(e.Stderr) > 0 {
				os.Stderr.Write(e.Stderr)
			}
			return err
		}
		if config.Actions&LifecycleActionDelete != 0 {
			defer func() {
				// runtime error or the container is already deleted
				if _, err := r.State(); err != nil {
					return
				}
				err := WaitingForStatus(r, LifecycleStatusCreated|LifecycleStatusStopped, time.Second*10, time.Second*1)
				if err == nil {
					r.Delete()
				} else {
					os.Stderr.WriteString("failed to delete the container\n")
					os.Stderr.WriteString(err.Error())
				}
			}()
		}
	}

	if config.PostCreate != nil {
		if err := config.PostCreate(&r); err != nil {
			return err
		}
	}

	if config.Actions&LifecycleActionStart != 0 {
		err := r.Start()
		if err != nil {
			os.Stderr.WriteString("failed to start the container\n")
			if e, ok := err.(*exec.ExitError); ok && len(e.Stderr) > 0 {
				os.Stderr.Write(e.Stderr)
			}
			return err
		}
	}

	if config.PreDelete != nil {
		if err := config.PreDelete(&r); err != nil {
			return err
		}
	}

	if config.Actions&LifecycleActionDelete != 0 {
		err := r.Delete()
		if err != nil {
			os.Stderr.WriteString("failed to delete the container\n")
			if e, ok := err.(*exec.ExitError); ok && len(e.Stderr) > 0 {
				os.Stderr.Write(e.Stderr)
			}
			return err
		}
	}

	if config.PostDelete != nil {
		if err := config.PostDelete(&r); err != nil {
			return err
		}
	}
	return nil
}
