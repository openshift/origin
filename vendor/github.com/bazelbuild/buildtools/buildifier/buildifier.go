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

// Buildifier, a tool to parse and format BUILD files.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/bazelbuild/buildtools/build"
	"github.com/bazelbuild/buildtools/differ"
	"github.com/bazelbuild/buildtools/tables"
)

var buildifierVersion = "redacted"
var buildScmRevision = "redacted"

var (
	// Undocumented; for debugging.
	showlog = flag.Bool("showlog", false, "show log in check mode")

	vflag         = flag.Bool("v", false, "print verbose information on standard error")
	dflag         = flag.Bool("d", false, "alias for -mode=diff")
	mode          = flag.String("mode", "", "formatting mode: check, diff, or fix (default fix)")
	path          = flag.String("path", "", "assume BUILD file has this path relative to the workspace directory")
	tablesPath    = flag.String("tables", "", "path to JSON file with custom table definitions which will replace the built-in tables")
	addTablesPath = flag.String("add_tables", "", "path to JSON file with custom table definitions which will be merged with the built-in tables")
	version       = flag.Bool("version", false, "Print the version of buildifier")

	// Debug flags passed through to rewrite.go
	allowSort = stringList("allowsort", "additional sort contexts to treat as safe")
	disable   = stringList("buildifier_disable", "list of buildifier rewrites to disable")

	// Experimental flags
	formatBzlFiles = flag.Bool("format_bzl", false, "format bzl-specific blocks (experimental)")
)

func stringList(name, help string) func() []string {
	f := flag.String(name, "", help)
	return func() []string {
		return strings.Split(*f, ",")
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage: buildifier [-d] [-v] [-mode=mode] [-path=path] [files...]

Buildifier applies a standard formatting to the named BUILD files.
The mode flag selects the processing: check, diff, fix, or print_if_changed.
In check mode, buildifier prints a list of files that need reformatting.
In diff mode, buildifier shows the diffs that it would make.
In fix mode, buildifier updates the files that need reformatting and,
if the -v flag is given, prints their names to standard error.
In print_if_changed mode, buildifier shows the file contents it would write.
The default mode is fix. -d is an alias for -mode=diff.

If no files are listed, buildifier reads a BUILD file from standard input. In
fix mode, it writes the reformatted BUILD file to standard output, even if no
changes are necessary.

Buildifier's reformatting depends in part on the path to the file relative
to the workspace directory. Normally buildifier deduces that path from the
file names given, but the path can be given explicitly with the -path
argument. This is especially useful when reformatting standard input,
or in scripts that reformat a temporary copy of a file.
`)
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()

	if *version {
		fmt.Printf("buildifier version: %s \n", buildifierVersion)
		fmt.Printf("buildifier scm revision: %s \n", buildScmRevision)

		if len(args) == 0 {
			os.Exit(0)
		}
	}

	// Pass down debug flags into build package
	build.DisableRewrites = disable()
	build.AllowSort = allowSort()

	tables.FormatBzlFiles = *formatBzlFiles

	if *dflag {
		if *mode != "" {
			fmt.Fprintf(os.Stderr, "buildifier: cannot specify both -d and -mode flags\n")
			os.Exit(2)
		}
		*mode = "diff"
	}

	// Check mode.
	switch *mode {
	default:
		fmt.Fprintf(os.Stderr, "buildifier: unrecognized mode %s; valid modes are check, diff, fix\n", *mode)
		os.Exit(2)

	case "":
		*mode = "fix"

	case "check", "diff", "fix", "print_if_changed":
		// ok
	}

	// If the path flag is set, must only be formatting a single file.
	// It doesn't make sense for multiple files to have the same path.
	if (*path != "" || *mode == "print_if_changed") && len(args) > 1 {
		fmt.Fprintf(os.Stderr, "buildifier: can only format one file when using -path flag or -mode=print_if_changed\n")
		os.Exit(2)
	}

	if *tablesPath != "" {
		if err := tables.ParseAndUpdateJSONDefinitions(*tablesPath, false); err != nil {
			fmt.Fprintf(os.Stderr, "buildifier: failed to parse %s for -tables: %s\n", *tablesPath, err)
			os.Exit(2)
		}
	}

	if *addTablesPath != "" {
		if err := tables.ParseAndUpdateJSONDefinitions(*addTablesPath, true); err != nil {
			fmt.Fprintf(os.Stderr, "buildifier: failed to parse %s for -add_tables: %s\n", *addTablesPath, err)
			os.Exit(2)
		}
	}

	diff = differ.Find()

	if len(args) == 0 || (len(args) == 1 && args[0] == "-") {
		// Read from stdin, write to stdout.
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "buildifier: reading stdin: %v\n", err)
			os.Exit(2)
		}
		if *mode == "fix" {
			*mode = "pipe"
		}
		processFile("stdin", data)
	} else {
		processFiles(args)
	}

	diff.Run()

	for _, file := range toRemove {
		os.Remove(file)
	}

	os.Exit(exitCode)
}

func processFiles(files []string) {
	// Decide how many file reads to run in parallel.
	// At most 100, and at most one per 10 input files.
	nworker := 100
	if n := (len(files) + 9) / 10; nworker > n {
		nworker = n
	}
	runtime.GOMAXPROCS(nworker + 1)

	// Start nworker workers reading stripes of the input
	// argument list and sending the resulting data on
	// separate channels. file[k] is read by worker k%nworker
	// and delivered on ch[k%nworker].
	type result struct {
		file string
		data []byte
		err  error
	}
	ch := make([]chan result, nworker)
	for i := 0; i < nworker; i++ {
		ch[i] = make(chan result, 1)
		go func(i int) {
			for j := i; j < len(files); j += nworker {
				file := files[j]
				data, err := ioutil.ReadFile(file)
				ch[i] <- result{file, data, err}
			}
		}(i)
	}

	// Process files. The processing still runs in a single goroutine
	// in sequence. Only the reading of the files has been parallelized.
	// The goal is to optimize for runs where most files are already
	// formatted correctly, so that reading is the bulk of the I/O.
	for i, file := range files {
		res := <-ch[i%nworker]
		if res.file != file {
			fmt.Fprintf(os.Stderr, "buildifier: internal phase error: got %s for %s", res.file, file)
			os.Exit(3)
		}
		if res.err != nil {
			fmt.Fprintf(os.Stderr, "buildifier: %v\n", res.err)
			exitCode = 3
			continue
		}
		processFile(file, res.data)
	}
}

// exitCode is the code to use when exiting the program.
// The codes used by buildifier are:
//
// 0: success, everything went well
// 1: syntax errors in input
// 2: usage errors: invoked incorrectly
// 3: unexpected runtime errors: file I/O problems or internal bugs
// 4: check mode failed (reformat is needed)
var exitCode = 0

// toRemove is a list of files to remove before exiting.
var toRemove []string

// diff is the differ to use when *mode == "diff".
var diff *differ.Differ

// processFile processes a single file containing data.
// It has been read from filename and should be written back if fixing.
func processFile(filename string, data []byte) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintf(os.Stderr, "buildifier: %s: internal error: %v\n", filename, err)
			exitCode = 3
		}
	}()

	f, err := build.Parse(filename, data)
	if err != nil {
		// Do not use buildifier: prefix on this error.
		// Since it is a parse error, it begins with file:line:
		// and we want that to be the first thing in the error.
		fmt.Fprintf(os.Stderr, "%v\n", err)
		if exitCode < 1 {
			exitCode = 1
		}
		return
	}

	if *path != "" {
		f.Path = *path
	}
	beforeRewrite := build.Format(f)
	var info build.RewriteInfo
	build.Rewrite(f, &info)
	ndata := build.Format(f)

	switch *mode {
	case "check":
		// check mode: print names of files that need formatting.
		if !bytes.Equal(data, ndata) {
			// Print:
			//	name # list of what changed
			reformat := ""
			if !bytes.Equal(data, beforeRewrite) {
				reformat = " reformat"
			}
			log := ""
			if len(info.Log) > 0 && *showlog {
				sort.Strings(info.Log)
				var uniq []string
				last := ""
				for _, s := range info.Log {
					if s != last {
						last = s
						uniq = append(uniq, s)
					}
				}
				log = " " + strings.Join(uniq, " ")
			}
			fmt.Printf("%s #%s %s%s\n", filename, reformat, &info, log)
			exitCode = 4
		}
		return

	case "diff":
		// diff mode: run diff on old and new.
		if bytes.Equal(data, ndata) {
			return
		}
		outfile, err := writeTemp(ndata)
		if err != nil {
			fmt.Fprintf(os.Stderr, "buildifier: %v\n", err)
			exitCode = 3
			return
		}
		infile := filename
		if filename == "" {
			// data was read from standard filename.
			// Write it to a temporary file so diff can read it.
			infile, err = writeTemp(data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "buildifier: %v\n", err)
				exitCode = 3
				return
			}
		}
		diff.Show(infile, outfile)

	case "pipe":
		// pipe mode - reading from stdin, writing to stdout.
		// ("pipe" is not from the command line; it is set above in main.)
		os.Stdout.Write(ndata)
		return

	case "fix":
		// fix mode: update files in place as needed.
		if bytes.Equal(data, ndata) {
			return
		}

		err := ioutil.WriteFile(filename, ndata, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "buildifier: %s\n", err)
			exitCode = 3
			return
		}

		if *vflag {
			fmt.Fprintf(os.Stderr, "fixed %s\n", filename)
		}
	case "print_if_changed":
		if bytes.Equal(data, ndata) {
			return
		}

		if _, err := os.Stdout.Write(ndata); err != nil {
			fmt.Fprintf(os.Stderr, "buildifier: error writing output: %v\n", err)
			exitCode = 3
			return
		}
	}
}

// writeTemp writes data to a temporary file and returns the name of the file.
func writeTemp(data []byte) (file string, err error) {
	f, err := ioutil.TempFile("", "buildifier-tmp-")
	if err != nil {
		return "", fmt.Errorf("creating temporary file: %v", err)
	}
	name := f.Name()
	toRemove = append(toRemove, name)
	defer f.Close()
	_, err = f.Write(data)
	if err != nil {
		return "", fmt.Errorf("writing temporary file: %v", err)
	}
	return name, nil
}
