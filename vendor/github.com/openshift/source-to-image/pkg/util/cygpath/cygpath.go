package cygpath

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// UsingCygwinGit indicates whether we believe the host's git utility is from
// Cygwin (expects Windows paths as /cygdrive/c/dir/file) or not (expects paths
// in host-native format).
var UsingCygwinGit = isUsingCygwinGit()

func isUsingCygwinGit() bool {
	if runtime.GOOS == "windows" {
		// If we find the cygpath utility (which translates between UNIX-style and
		// Windows-style paths) in the same directory as git, assume we're using
		// Cygwin.
		cygpath, err := exec.LookPath("cygpath")
		if err != nil {
			return false
		}
		var git string
		git, err = exec.LookPath("git")
		if err == nil && filepath.Dir(cygpath) == filepath.Dir(git) {
			return true
		}
	}
	return false
}

// ToSlashCygwin converts a path to a format suitable for sending to a Cygwin
// binary - `/dir/file` on UNIX-style systems; `c:\dir\file` on Windows without
// Cygwin; `/cygdrive/c/dir/file` on Windows with Cygwin.
func ToSlashCygwin(path string) (string, error) {
	cmd := exec.Command("cygpath", path)
	out, err := cmd.Output()
	return strings.TrimRight(string(out), "\n"), err
}
