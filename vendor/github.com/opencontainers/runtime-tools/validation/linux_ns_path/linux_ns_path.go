package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	"github.com/mndrix/tap-go"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/specerror"
	"github.com/opencontainers/runtime-tools/validation/util"
)

func waitForState(stateCheckFunc func() error) error {
	timeout := 3 * time.Second
	alarm := time.After(timeout)
	ticker := time.Tick(200 * time.Millisecond)
	for {
		select {
		case <-alarm:
			return fmt.Errorf("failed to reach expected state within %v", timeout)
		case <-ticker:
			if err := stateCheckFunc(); err == nil {
				return nil
			}
		}
	}
}

func checkNamespacePath(t *tap.T, unsharePid int, ns string) error {
	testNsPath := fmt.Sprintf("/proc/%d/ns/%s", os.Getpid(), ns)
	testNsInode, err := os.Readlink(testNsPath)
	if err != nil {
		out, err2 := exec.Command("sh", "-c", fmt.Sprintf("ls -la /proc/%d/ns/", os.Getpid())).CombinedOutput()
		return fmt.Errorf("cannot read namespace link for the test process: %s\n%v\n%v", err, err2, string(out))
	}

	var errNsPath error

	unshareNsPath := ""
	unshareNsInode := ""

	doCheckNamespacePath := func() error {
		specialChildren := ""
		if ns == "pid" {
			// Unsharing pidns does not move the process into the new
			// pidns but the next forked process. 'unshare' is called with
			// '--fork' so the pidns will be fully created and populated
			// with a pid 1.
			//
			// However, finding out the pid of the child process is not
			// trivial: it would require to parse
			// /proc/$pid/task/$tid/children but that only works on kernels
			// with CONFIG_PROC_CHILDREN (not all distros have that).
			//
			// It is easier to look at /proc/$pid/ns/pid_for_children on
			// the parent process. Available since Linux 4.12.
			specialChildren = "_for_children"
		}
		unshareNsPath = fmt.Sprintf("/proc/%d/ns/%s", unsharePid, ns+specialChildren)
		unshareNsInode, err = os.Readlink(unshareNsPath)
		if err != nil {
			errNsPath = fmt.Errorf("cannot read namespace link for the unshare process: %s", err)
			return errNsPath
		}

		if testNsInode == unshareNsInode {
			errNsPath = fmt.Errorf("expected: %q, found: %q", testNsInode, unshareNsInode)
			return errNsPath
		}

		return nil
	}

	// Since it takes some time until unshare switched to the new namespace,
	// we should make a loop to check for the result up to 3 seconds.
	if err := waitForState(doCheckNamespacePath); err != nil {
		// we should return errNsPath instead of err, because errNsPath is what
		// returned from the actual test function doCheckNamespacePath(), not
		// waitForState().
		return errNsPath
	}

	g, err := util.GetDefaultGenerator()
	if err != nil {
		return fmt.Errorf("cannot get the default generator: %v", err)
	}

	rtns := util.GetRuntimeToolsNamespace(ns)
	g.AddOrReplaceLinuxNamespace(rtns, unshareNsPath)

	return util.RuntimeOutsideValidate(g, t, func(config *rspec.Spec, t *tap.T, state *rspec.State) error {
		containerNsPath := fmt.Sprintf("/proc/%d/ns/%s", state.Pid, ns)
		containerNsInode, err := os.Readlink(containerNsPath)
		if err != nil {
			out, err2 := exec.Command("sh", "-c", fmt.Sprintf("ls -la /proc/%d/ns/", state.Pid)).CombinedOutput()
			return fmt.Errorf("cannot read namespace link for the container process: %s\n%v\n%v", err, err2, out)
		}
		if containerNsInode != unshareNsInode {
			return fmt.Errorf("expected: %q, found: %q", unshareNsInode, containerNsInode)
		}
		return nil
	})
}

func testNamespacePath(t *tap.T, ns string, unshareOpt string) error {
	// Calling 'unshare' (part of util-linux) is easier than doing it from
	// Golang: mnt namespaces cannot be unshared from multithreaded
	// programs.
	cmd := exec.Command("unshare", unshareOpt, "--fork", "sleep", "10000")
	// We shoud set Setpgid to true, to be able to allow the unshare process
	// as well as its child processes to be killed by a single kill command.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("cannot run unshare: %s", err)
	}
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		cmd.Wait()
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}()
	if cmd.Process == nil {
		return fmt.Errorf("process failed to start")
	}

	return checkNamespacePath(t, cmd.Process.Pid, ns)
}

func main() {
	t := tap.New()
	t.Header(0)

	cases := []struct {
		name       string
		unshareOpt string
	}{
		{"ipc", "--ipc"},
		{"mnt", "--mount"},
		{"net", "--net"},
		{"pid", "--pid"},
		{"uts", "--uts"},
	}

	for _, c := range cases {
		if "linux" != runtime.GOOS {
			t.Skip(1, fmt.Sprintf("linux-specific namespace test: %s", c))
		}

		err := testNamespacePath(t, c.name, c.unshareOpt)
		t.Ok(err == nil, fmt.Sprintf("set %s namespace by path", c.name))
		if err != nil {
			rfcError, errRfc := specerror.NewRFCError(specerror.NSProcInPath, err, rspec.Version)
			if errRfc != nil {
				continue
			}
			diagnostic := map[string]string{
				"actual":         fmt.Sprintf("err == %v", err),
				"expected":       "err == nil",
				"namespace type": c.name,
				"level":          rfcError.Level.String(),
				"reference":      rfcError.Reference,
			}
			t.YAML(diagnostic)
		}
	}

	t.AutoPlan()
}
