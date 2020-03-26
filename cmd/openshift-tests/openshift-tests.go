package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	errorsutil "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog"
)

func main() {
	streams := genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}

	testBinaries, err := getTestBinaries(streams)
	if err != nil {
		klog.Fatal(err)
	}
	fmt.Fprintf(streams.ErrOut, "The following compatible plugins are available:\n%v\n\n", strings.Join(testBinaries, "\n"))

	for _, testBinary := range testBinaries {
		testCommand := exec.Command(testBinary, os.Args[1:]...)
		testCommand.Stdout = streams.Out
		testCommand.Stderr = streams.ErrOut
		testCommand.Stdin = streams.In

		if err := testCommand.Run(); err != nil {
			klog.Fatal(err)
		}
	}
}

func getTestBinaries(streams genericclioptions.IOStreams) ([]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}
	executablePath, err := os.Executable()
	if err != nil {
		log.Println(err)
	}
	executableDir := filepath.Dir(executablePath)
	testBinaryDirectories := []string{}
	testBinaryDirectories = append(testBinaryDirectories, cwd)
	testBinaryDirectories = append(testBinaryDirectories, executableDir)
	testBinaryDirectories = append(testBinaryDirectories, filepath.SplitList(os.Getenv("PATH"))...)

	warnings := 0
	errors := []error{}
	testBinaries := sets.String{}
	verifier := &CommandOverrideVerifier{
		//root:        cmd.Root(),
		seenPlugins: make(map[string]string),
	}
	for _, dir := range uniquePathsList(testBinaryDirectories) {
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			if _, ok := err.(*os.PathError); ok {
				fmt.Fprintf(streams.ErrOut, "Unable read directory %q from your PATH: %v. Skipping...\n", dir, err)
				continue
			}

			errors = append(errors, fmt.Errorf("error: unable to read directory %q in your PATH: %v", dir, err))
			continue
		}

		for _, f := range files {
			if f.IsDir() {
				continue
			}
			if !nameMatchesTest(f.Name()) {
				continue
			}

			testBinaryPath := filepath.Join(dir, f.Name())
			isSymlink, err := evalSymlink(testBinaryPath)
			if err != nil {
				log.Println(err)
			}
			if testBinaries.Has(testBinaryPath) || isSymlink {
				continue
			}
			testBinaries.Insert(testBinaryPath)

			fmt.Fprintf(streams.ErrOut, "%s\n", testBinaryPath)
			if errs := verifier.Verify(streams, filepath.Join(dir, f.Name())); len(errs) != 0 {
				for _, err := range errs {
					fmt.Fprintf(streams.ErrOut, "  - %s\n", err)
					warnings++
				}
			}
		}
	}
	if warnings > 0 {
		if warnings == 1 {
			errors = append(errors, fmt.Errorf("error: one plugin warning was found"))
		} else {
			errors = append(errors, fmt.Errorf("error: %v plugin warnings were found", warnings))
		}
	}
	if len(testBinaries) == 0 {
		errors = append(errors, fmt.Errorf("error: unable to find any kubectl plugins in your PATH"))
	}
	if len(errors) > 0 {
		return nil, errorsutil.NewAggregate(errors)
	}

	return testBinaries.List(), nil
}

type CommandOverrideVerifier struct {
	//root        *cobra.Command
	seenPlugins map[string]string
}

// evalSymlink returns true if provided path is a symlink
func evalSymlink(path string) (bool, error) {
	link, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false, err
	}
	if len(link) != 0 {
		if link != path {
			return true, nil
		}
	}
	return false, nil
}

// Verify implements PathVerifier and determines if a given path
// is valid depending on whether or not it overwrites an existing
// kubectl command path, or a previously seen plugin.
func (v *CommandOverrideVerifier) Verify(streams genericclioptions.IOStreams, path string) []error {
	//if v.root == nil {
	//	return []error{fmt.Errorf("unable to verify path with nil root")}
	//}

	// extract the plugin binary name
	segs := strings.Split(path, "/")
	binName := segs[len(segs)-1]

	cmdPath := strings.Split(binName, "-")
	if len(cmdPath) > 1 {
		// the first argument is always "kubectl" for a plugin binary
		cmdPath = cmdPath[1:]
	}

	errors := []error{}

	if isExec, err := isExecutable(path); err == nil && !isExec {
		errors = append(errors, fmt.Errorf("warning: %s identified as a kubectl plugin, but it is not executable", path))
	} else if err != nil {
		errors = append(errors, fmt.Errorf("error: unable to identify %s as an executable file: %v", path, err))
	}

	if existingPath, ok := v.seenPlugins[binName]; ok {
		fmt.Fprintf(streams.ErrOut, "warning: %s is overshadowed by a similarly named plugin: %s\n", path, existingPath)
	} else {
		v.seenPlugins[binName] = path
	}

	//if cmd, _, err := v.root.Find(cmdPath); err == nil {
	//	errors = append(errors, fmt.Errorf("warning: %s overwrites existing command: %q", binName, cmd.CommandPath()))
	//}

	return errors
}

func isExecutable(fullPath string) (bool, error) {
	info, err := os.Stat(fullPath)
	if err != nil {
		return false, err
	}

	if m := info.Mode(); !m.IsDir() && m&0111 != 0 {
		return true, nil
	}

	return false, nil
}

// uniquePathsList deduplicates a given slice of strings without
// sorting or otherwise altering its order in any way.
func uniquePathsList(paths []string) []string {
	seen := map[string]bool{}
	newPaths := []string{}
	for _, p := range paths {
		if seen[p] {
			continue
		}
		seen[p] = true
		newPaths = append(newPaths, p)
	}
	return newPaths
}

func nameMatchesTest(filepath string) bool {
	for _, prefix := range []string{"openshift-tests"} {
		if !strings.HasPrefix(filepath, prefix+"-") {
			continue
		}
		return true
	}

	return false
}
