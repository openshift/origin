package rsync

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

var (
	testRsyncCommand = []string{"rsync", "--version"}
	testTarCommand   = []string{"tar", "--version"}
)

// executeWithLogging will execute a command and log its output
func executeWithLogging(e executor, cmd []string) error {
	w := &bytes.Buffer{}
	err := e.Execute(cmd, nil, w, w)
	glog.V(4).Infof("%s", w.String())
	glog.V(4).Infof("error: %v", err)
	return err
}

// isWindows returns true if the current platform is windows
func isWindows() bool {
	return runtime.GOOS == "windows"
}

// hasLocalRsync returns true if rsync is in current exec path
func hasLocalRsync() bool {
	_, err := exec.LookPath("rsync")
	if err != nil {
		return false
	}
	return true
}

func isExitError(err error) bool {
	if err == nil {
		return false
	}
	_, exitErr := err.(*exec.ExitError)
	return exitErr
}

func checkRsync(e executor) error {
	return executeWithLogging(e, testRsyncCommand)
}

func checkTar(e executor) error {
	return executeWithLogging(e, testTarCommand)
}

func rsyncFlagsFromOptions(o *RsyncOptions) []string {
	flags := []string{}
	if o.Quiet {
		flags = append(flags, "-q")
	} else {
		flags = append(flags, "-v")
	}
	if o.Delete {
		flags = append(flags, "--delete")
	}
	if o.Compress {
		flags = append(flags, "-z")
	}
	if len(o.RsyncInclude) > 0 {
		for _, include := range o.RsyncInclude {
			flags = append(flags, fmt.Sprintf("--include=%s", include))
		}
	}
	if len(o.RsyncExclude) > 0 {
		for _, exclude := range o.RsyncExclude {
			flags = append(flags, fmt.Sprintf("--exclude=%s", exclude))
		}
	}
	if o.RsyncProgress {
		flags = append(flags, "--progress")
	}
	if o.RsyncNoPerms {
		flags = append(flags, "--no-perms")
	}
	return flags
}

func rsyncSpecificFlags(o *RsyncOptions) []string {
	flags := []string{}
	if len(o.RsyncInclude) > 0 {
		flags = append(flags, "--include")
	}
	if len(o.RsyncExclude) > 0 {
		flags = append(flags, "--exclude")
	}
	if o.RsyncProgress {
		flags = append(flags, "--progress")
	}
	if o.RsyncNoPerms {
		flags = append(flags, "--no-perms")
	}
	if o.Compress {
		flags = append(flags, "-z")
	}
	return flags
}

type podAPIChecker struct {
	client    kclientset.Interface
	namespace string
	podName   string
}

// CheckPods will check if pods exists in the provided context
func (p podAPIChecker) CheckPod() error {
	_, err := p.client.Core().Pods(p.namespace).Get(p.podName, metav1.GetOptions{})
	return err
}
