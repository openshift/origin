package discovery

import (
	"os/exec"
	"runtime"
)

// ----------------------------------------------------------
// Determine what we need to about the OS
func (env *Environment) DiscoverOperatingSystem() {
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("systemctl"); err == nil {
			env.HasSystemd = true
		}
		if _, err := exec.LookPath("/bin/bash"); err == nil {
			env.HasBash = true
		}
	}
}
