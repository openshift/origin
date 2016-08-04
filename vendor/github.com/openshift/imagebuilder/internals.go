package imagebuilder

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// hasEnvName returns true if the provided environment contains the named ENV var.
func hasEnvName(env []string, name string) bool {
	for _, e := range env {
		if strings.HasPrefix(e, name+"=") {
			return true
		}
	}
	return false
}

// platformSupports is a short-term function to give users a quality error
// message if a Dockerfile uses a command not supported on the platform.
func platformSupports(command string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	switch command {
	case "expose", "user", "stopsignal", "arg":
		return fmt.Errorf("The daemon on this platform does not support the command '%s'", command)
	}
	return nil
}

func handleJSONArgs(args []string, attributes map[string]bool) []string {
	if len(args) == 0 {
		return []string{}
	}

	if attributes != nil && attributes["json"] {
		return args
	}

	// literal string command, not an exec array
	return []string{strings.Join(args, " ")}
}

// makeAbsolute ensures that the provided path is absolute.
func makeAbsolute(dest, workingDir string) string {
	// Twiddle the destination when its a relative path - meaning, make it
	// relative to the WORKINGDIR
	if !filepath.IsAbs(dest) {
		hasSlash := strings.HasSuffix(dest, string(os.PathSeparator)) || strings.HasSuffix(dest, string(os.PathSeparator)+".")
		dest = filepath.Join(string(os.PathSeparator), filepath.FromSlash(workingDir), dest)

		// Make sure we preserve any trailing slash
		if hasSlash {
			dest += string(os.PathSeparator)
		}
	}
	return dest
}
