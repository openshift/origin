/*
Copyright 2016 Google Inc. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

// Package differ determines how to invoke diff in the given environment.
package differ

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Invocation of different diff commands, according to environment variables.

// A Differ describes how to invoke diff.
type Differ struct {
	Cmd       string   // command
	MultiDiff bool     // diff accepts list of multiple pairs
	Args      []string // accumulated arguments
}

// run runs the given command with args.
func (d *Differ) run(command string, args ...string) {
	// The special diff command ":" means don't run anything.
	if d.Cmd == ":" {
		return
	}

	// Pass args to bash and reference with $@ to avoid shell injection in args.
	var cmd *exec.Cmd
	if command == "FC" {
		cmd = exec.Command(command, "/T")
	} else {
		cmd = exec.Command("/bin/bash", "-c", command+` "$@"`, "--")
	}
	cmd.Args = append(cmd.Args, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		// Couldn't even start bash. Worth reporting.
		fmt.Fprintf(os.Stderr, "buildifier: %s: %v\n", command, err)
		return
	}

	// Assume bash reported anything else worth reporting.
	// As long as the program started (above), we don't care about the
	// exact exit status. In the most common case, the diff command
	// will exit 1, because there are diffs, causing bash to exit 1.
	cmd.Wait()
}

// Show diffs old and new.
// For a single-pair diff program, Show runs the diff program before returning.
// For a multi-pair diff program, Show records the pair for later use by Run.
func (d *Differ) Show(old, new string) {
	if !d.MultiDiff {
		d.run(d.Cmd, old, new)
		return
	}

	d.Args = append(d.Args, ":", old, new)
}

// Run runs any pending diffs.
// For a single-pair diff program, Show already ran diff; Run is a no-op.
// For a multi-pair diff program, Run displays the diffs queued by Show.
func (d *Differ) Run() {
	if !d.MultiDiff {
		return
	}

	if len(d.Args) == 0 {
		return
	}
	d.run(d.Cmd, d.Args...)
}

// Find returns the differ to use, using various environment variables.
func Find() *Differ {
	d := &Differ{}
	if cmd := os.Getenv("BUILDIFIER_DIFF"); cmd != "" {
		d.Cmd = cmd
	}

	// Load MultiDiff setting from environment.
	knowMultiDiff := false
	if md := os.Getenv("BUILDIFIER_MULTIDIFF"); md == "0" || md == "1" {
		d.MultiDiff = md == "1"
		knowMultiDiff = true
	}

	if d.Cmd != "" {
		if !knowMultiDiff {
			lower := strings.ToLower(d.Cmd)
			d.MultiDiff = strings.Contains(lower, "tkdiff") &&
				isatty(1) && os.Getenv("DISPLAY") != ""
		}
	} else {
		if !knowMultiDiff {
			d.MultiDiff = isatty(1) && os.Getenv("DISPLAY") != ""
		}
		if d.MultiDiff {
			d.Cmd = "tkdiff"
		} else {
			if runtime.GOOS == "windows" {
				d.Cmd = "FC"
			} else {
				d.Cmd = "diff"
			}
		}
	}
	return d
}
