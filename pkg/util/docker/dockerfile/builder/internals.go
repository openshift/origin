package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// isURL returns true if the string appears to be a URL.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// exportEnv creates an export statement for a shell that contains all of the
// provided environment.
func exportEnv(env []string) string {
	if len(env) == 0 {
		return ""
	}
	out := "export"
	for _, e := range env {
		out += " " + bashQuote(e)
	}
	return out + "; "
}

// bashQuote escapes the provided string and surrounds it with double quotes.
// TODO: verify that these are all we have to escape.
func bashQuote(env string) string {
	out := []rune{'"'}
	for _, r := range env {
		switch r {
		case '$', '\\', '"':
			out = append(out, '\\', r)
		default:
			out = append(out, r)
		}
	}
	out = append(out, '"')
	return string(out)
}

// hasEnvName returns true if the provided enviroment contains the named ENV var.
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
