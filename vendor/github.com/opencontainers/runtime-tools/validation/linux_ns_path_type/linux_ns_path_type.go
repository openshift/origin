package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"syscall"

	"github.com/mndrix/tap-go"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/specerror"
	"github.com/opencontainers/runtime-tools/validation/util"
)

func checkNSPathMatchType(t *tap.T, ns, wrongNs string) error {
	// Deliberately set ns path with a wrong namespace, to check if the runtime
	// returns error when running with the wrong namespace path.
	unshareNsPath := fmt.Sprintf("/proc/self/ns/%s", wrongNs)

	g, err := util.GetDefaultGenerator()
	if err != nil {
		return fmt.Errorf("cannot get default config from generator: %v", err)
	}

	rtns := util.GetRuntimeToolsNamespace(ns)
	g.AddOrReplaceLinuxNamespace(rtns, unshareNsPath)

	err = util.RuntimeOutsideValidate(g, t, nil)

	t.Ok(err != nil, fmt.Sprintf("got error when setting a wrong namespace path %q with type %s", unshareNsPath, rtns))
	if err == nil {
		rfcError, errRfc := specerror.NewRFCError(specerror.NSPathMatchTypeError,
			fmt.Errorf("got no error when setting a wrong namespace path %q with type %s", unshareNsPath, rtns),
			rspec.Version)
		if errRfc != nil {
			return fmt.Errorf("cannot get new rfcError: %v", errRfc)
		}
		diagnostic := map[string]string{
			"expected":       fmt.Sprintf("err == %v", err),
			"actual":         "err == nil",
			"namespace type": rtns,
			"level":          rfcError.Level.String(),
			"reference":      rfcError.Reference,
		}
		t.YAML(diagnostic)

		return fmt.Errorf("cannot validate path with wrong type")
	}

	return nil
}

func testNSPathMatchType(t *tap.T, ns, unshareOpt, wrongNs string) error {
	// Calling 'unshare' (part of util-linux) is easier than doing it from
	// Golang: mnt namespaces cannot be unshared from multithreaded
	// programs.
	cmd := exec.Command("unshare", unshareOpt, "--fork", "sleep", "10000")
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

	return checkNSPathMatchType(t, ns, wrongNs)
}

func main() {
	t := tap.New()
	t.Header(0)

	cases := []struct {
		name       string
		unshareOpt string
		wrongname  string
	}{
		{"cgroup", "--cgroup", "ipc"},
		{"ipc", "--ipc", "mnt"},
		{"mnt", "--mount", "net"},
		{"net", "--net", "pid"},
		{"pid", "--pid", "user"},
		{"user", "--user", "uts"},
		{"uts", "--uts", "cgroup"},
	}

	for _, c := range cases {
		if "linux" != runtime.GOOS {
			t.Skip(1, fmt.Sprintf("linux-specific namespace test: %s", c))
		}

		err := testNSPathMatchType(t, c.name, c.unshareOpt, c.wrongname)
		t.Ok(err == nil, fmt.Sprintf("namespace path matches with type %s", c.name))
	}

	t.AutoPlan()
}
