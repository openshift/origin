package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mndrix/tap-go"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/specerror"
	"github.com/opencontainers/runtime-tools/validation/util"
)

func printDiag(t *tap.T, diagActual, diagExpected, diagNsType string, errNs error) {
	specErr := specerror.NewError(specerror.NSNewNSWithoutPath,
		errNs, rspec.Version)
	diagnostic := map[string]string{
		"actual":         diagActual,
		"expected":       diagExpected,
		"namespace type": diagNsType,
		"level":          specErr.(*specerror.Error).Err.Level.String(),
		"reference":      specErr.(*specerror.Error).Err.Reference,
	}
	t.YAML(diagnostic)
}

func testNamespaceNoPath(t *tap.T) error {
	var errNs error
	diagActual := ""
	diagExpected := ""
	diagNsType := ""

	// To be able to print out diagnostics for all kinds of error cases
	// at the end of the tests, we make use of defer function. To do that,
	// each error handling routine should set diagActual, diagExpected,
	// diagNsType, and errNs, before returning an error.
	defer func() {
		if errNs != nil {
			printDiag(t, diagActual, diagExpected, diagNsType, errNs)
		}
	}()

	hostNsPath := fmt.Sprintf("/proc/%d/ns", os.Getpid())
	hostNsInodes := map[string]string{}

	for _, nsName := range util.ProcNamespaces {
		nsPathAbs := filepath.Join(hostNsPath, nsName)
		nsInode, err := os.Readlink(nsPathAbs)
		if err != nil {
			errNs = fmt.Errorf("cannot resolve symlink %q: %v", nsPathAbs, err)
			diagActual = fmt.Sprintf("err == %v", errNs)
			diagExpected = "err == nil"
			diagNsType = nsName
			return errNs
		}
		hostNsInodes[nsName] = nsInode
	}

	g, err := util.GetDefaultGenerator()
	if err != nil {
		errNs = fmt.Errorf("cannot get the default generator: %v", err)
		diagActual = fmt.Sprintf("err == %v", errNs)
		diagExpected = "err == nil"
		// NOTE: we don't have a namespace type
		return errNs
	}

	// As the namespaces, cgroups and user, are not set by GetDefaultGenerator(),
	// others are set by default. We just set them explicitly to avoid confusion.
	g.AddOrReplaceLinuxNamespace("cgroup", "")
	g.AddOrReplaceLinuxNamespace("ipc", "")
	g.AddOrReplaceLinuxNamespace("mount", "")
	g.AddOrReplaceLinuxNamespace("network", "")
	g.AddOrReplaceLinuxNamespace("pid", "")
	g.AddOrReplaceLinuxNamespace("user", "")
	g.AddOrReplaceLinuxNamespace("uts", "")

	// For user namespaces, we need to set uid/gid maps to create a container
	g.AddLinuxUIDMapping(uint32(1000), uint32(0), uint32(1000))
	g.AddLinuxGIDMapping(uint32(1000), uint32(0), uint32(1000))

	err = util.RuntimeOutsideValidate(g, t, func(config *rspec.Spec, t *tap.T, state *rspec.State) error {
		containerNsPath := fmt.Sprintf("/proc/%d/ns", state.Pid)

		for _, nsName := range util.ProcNamespaces {
			nsPathAbs := filepath.Join(containerNsPath, nsName)
			nsInode, err := os.Readlink(nsPathAbs)
			if err != nil {
				errNs = fmt.Errorf("cannot resolve symlink %q: %v", nsPathAbs, err)
				diagActual = fmt.Sprintf("err == %v", errNs)
				diagExpected = "err == nil"
				diagNsType = nsName
				return errNs
			}

			t.Ok(hostNsInodes[nsName] != nsInode, fmt.Sprintf("create namespace %s without path", nsName))
			if hostNsInodes[nsName] == nsInode {
				// NOTE: for such inode match cases, we should print out diagnostics
				// for each case, not only at the end of tests. So we should simply
				// call once printDiag(), then continue testing next namespaces.
				// Thus we don't need to set diagActual, diagExpected, diagNsType, etc.
				printDiag(t, nsInode, fmt.Sprintf("!= %s", hostNsInodes[nsName]), nsName,
					fmt.Errorf("both namespaces for %s have the same inode %s", nsName, nsInode))
				continue
			}
		}

		return nil
	})
	if err != nil {
		errNs = fmt.Errorf("cannot run validation tests: %v", err)
	}

	return errNs
}

func main() {
	t := tap.New()
	t.Header(0)

	if "linux" != runtime.GOOS {
		t.Skip(1, fmt.Sprintf("linux-specific namespace test"))
	}

	err := testNamespaceNoPath(t)
	if err != nil {
		t.Fail(err.Error())
	}

	t.AutoPlan()
}
