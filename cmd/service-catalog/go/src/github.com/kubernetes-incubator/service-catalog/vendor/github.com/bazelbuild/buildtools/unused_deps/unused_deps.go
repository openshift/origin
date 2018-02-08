/*
Copyright 2017 Google Inc. All Rights Reserved.
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

// The unused_deps binary prints out buildozer commands for removing
// unused java dependencies from java_library bazel rules.
package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/bazelbuild/buildtools/build"
	"github.com/bazelbuild/buildtools/config"
	depspb "github.com/bazelbuild/buildtools/deps_proto"
	"github.com/bazelbuild/buildtools/edit"
	eapb "github.com/bazelbuild/buildtools/extra_actions_base_proto"
	"github.com/golang/protobuf/proto"
)

var (
	buildTool           = flag.String("build_tool", config.DefaultBuildTool, config.BuildToolHelp)
	extraActionFileName = flag.String("extra_action_file", "", config.ExtraActionFileNameHelp)
	outputFileName      = flag.String("output_file", "", "used only with extra_action_file")
	buildOptions        = stringList("extra_build_flags", "Extra build flags to use when building the targets.")

	blazeFlags = []string{"--tool_tag=unused_deps", "--keep_going", "--color=yes", "--curses=yes"}
)

func stringList(name, help string) func() []string {
	f := flag.String(name, "", help)
	return func() []string {
		if *f == "" {
			return nil
		}
		res := strings.Split(*f, ",")
		for i := range res {
			res[i] = strings.TrimSpace(res[i])
		}
		return res
	}
}

// getJarPath prints the path to the output jar file specified in the extra_action file at path.
func getJarPath(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	i := &eapb.ExtraActionInfo{}
	if err := proto.Unmarshal(data, i); err != nil {
		return "", err
	}
	ext, err := proto.GetExtension(i, eapb.E_JavaCompileInfo_JavaCompileInfo)
	if err != nil {
		return "", err
	}
	jci, ok := ext.(*eapb.JavaCompileInfo)
	if !ok {
		return "", errors.New("no JavaCompileInfo in " + path)
	}
	return jci.GetOutputjar(), nil
}

// writeUnusedDeps writes the labels of unused direct deps, one per line, to outputFileName.
func writeUnusedDeps(jarPath, outputFileName string) {
	depsPath := strings.Replace(jarPath, ".jar", ".jdeps", 1)
	paramsPath := jarPath + "-2.params"
	file, _ := os.Create(outputFileName)
	for dep := range unusedDeps(depsPath, directDepParams(paramsPath)) {
		file.WriteString(dep + "\n")
	}
}

func cmdWithStderr(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.Stderr = os.Stderr
	return cmd
}

// blazeInfo retrieves the blaze info value for a given key.
func blazeInfo(key string) (value string) {
	out, err := cmdWithStderr(*buildTool, "info", key).Output()
	if err != nil {
		log.Printf("'%s info %s' failed: %s", *buildTool, key, err)
	}
	return strings.TrimSpace(bytes.NewBuffer(out).String())
}

// inputFileName returns a blaze output file name from which to read input.
func inputFileName(blazeBin, pkg, ruleName, extension string) string {
	name := fmt.Sprintf("%s/%s/lib%s.%s", blazeBin, pkg, ruleName, extension) // *_library
	if _, err := os.Stat(name); err == nil {
		return name
	}
	// lazily let the caller handle it if this doesn't exist
	return fmt.Sprintf("%s/%s/%s.%s", blazeBin, pkg, ruleName, extension) // *_{binary,test}
}

// directDepParams returns --direct_dependency entries from paramsFileName (a jar-2.params file)
// as a map from jar files to labels.
func directDepParams(paramsFileName string) (depsByJar map[string]string) {
	depsByJar = make(map[string]string)
	data, err := ioutil.ReadFile(paramsFileName)
	if err != nil {
		log.Println(err)
		return depsByJar
	}
	// the classpath param exceeds MaxScanTokenSize, so we scan just this section:
	first := bytes.Index(data, []byte("--direct_dependency"))
	if first < 0 {
		return depsByJar
	}
	scanner := bufio.NewScanner(bytes.NewReader(data[first:]))
	for scanner.Scan() {
		if scanner.Text() == "--direct_dependency" {
			scanner.Scan()
			jar := scanner.Text()
			scanner.Scan()
			label := scanner.Text()
			depsByJar[jar] = label
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("reading %s: %s", paramsFileName, err)
	}
	return depsByJar
}

// unusedDeps returns a set of labels that are unused deps.
// It reads Dependencies proto messages from depsFileName (a jdeps file), which indicate deps used
// at compile time, and returns those values in the depsByJar map that aren't used at compile time.
func unusedDeps(depsFileName string, depsByJar map[string]string) (unusedDeps map[string]bool) {
	unusedDeps = make(map[string]bool)
	data, err := ioutil.ReadFile(depsFileName)
	if err != nil {
		log.Println(err)
		return unusedDeps
	}
	dependencies := &depspb.Dependencies{}
	if err := proto.Unmarshal(data, dependencies); err != nil {
		log.Println(err)
		return unusedDeps
	}
	for _, label := range depsByJar {
		unusedDeps[label] = true
	}
	for _, dependency := range dependencies.Dependency {
		if *dependency.Kind == depspb.Dependency_EXPLICIT {
			delete(unusedDeps, depsByJar[*dependency.Path])
		}
	}
	return unusedDeps
}

// parseBuildFile tries to read and parse the contents of buildFileName.
func parseBuildFile(buildFileName string) (buildFile *build.File, err error) {
	data, err := ioutil.ReadFile(buildFileName)
	if err != nil {
		return nil, err
	}
	return build.Parse(buildFileName, data)
}

// getDepsExpr tries to parse the content of buildFileName and return the deps Expr for ruleName.
func getDepsExpr(buildFileName string, ruleName string) build.Expr {
	buildFile, err := parseBuildFile(buildFileName)
	if buildFile == nil {
		log.Printf("%s when parsing %s", err, buildFileName)
		return nil
	}
	rule := edit.FindRuleByName(buildFile, ruleName)
	if rule == nil {
		log.Printf("%s not found in %s", ruleName, buildFileName)
		return nil
	}
	depsExpr := rule.Attr("deps")
	if depsExpr == nil {
		log.Printf("no deps attribute for %s in %s", ruleName, buildFileName)
	}
	return depsExpr
}

// hasRuntimeComment returns true if expr has an EOL comment containing the word "runtime".
// TODO(bazel-team): delete when this comment convention is extinct
func hasRuntimeComment(expr build.Expr) bool {
	for _, comment := range expr.Comment().Suffix {
		if strings.Contains(strings.ToLower(comment.Token), "runtime") {
			return true
		}
	}
	return false
}

// printCommands prints, for each key in the deps map, a buildozer command
// to remove that entry from the deps attribute of the rule identified by label.
// Returns true if at least one command was printed, or false otherwise.
func printCommands(label string, deps map[string]bool) (anyCommandPrinted bool) {
	buildFileName, pkg, ruleName := edit.InterpretLabel(label)
	depsExpr := getDepsExpr(buildFileName, ruleName)
	for _, li := range edit.AllLists(depsExpr) {
		for _, elem := range li.List {
			for dep := range deps {
				str, ok := elem.(*build.StringExpr)
				if ok && edit.LabelsEqual(str.Value, dep, pkg) {
					if hasRuntimeComment(str) {
						fmt.Printf("buildozer 'move deps runtime_deps %s' %s\n", str.Value, label)
					} else {
						fmt.Printf("buildozer 'remove deps %s' %s\n", str.Value, label)
					}
					anyCommandPrinted = true
				}
			}
		}
	}
	return anyCommandPrinted
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage: unused_deps TARGET...

For Java rules in TARGETs, prints commands to delete deps unused at compile time.
Note these may be used at run time; see documentation for more information.
`)
	os.Exit(2)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if *extraActionFileName != "" {
		jarPath, err := getJarPath(*extraActionFileName)
		if err != nil {
			log.Fatal(err)
		}
		writeUnusedDeps(jarPath, *outputFileName)
		return
	}
	targetPatterns := flag.Args()

	queryCmd := append([]string{"query"}, blazeFlags...)
	queryCmd = append(
		queryCmd, fmt.Sprintf("kind('(java|android)_*', %s)", strings.Join(targetPatterns, " + ")))

	log.Printf("running: %s %s", *buildTool, strings.Join(queryCmd, " "))
	queryOut, err := cmdWithStderr(*buildTool, queryCmd...).Output()
	if err != nil {
		log.Print(err)
	}
	if len(queryOut) == 0 {
		fmt.Fprintln(os.Stderr, "found no targets of kind (java|android)_*")
		usage()
	}

	buildCmd := append(append([]string{"build"}, blazeFlags...), config.DefaultExtraBuildFlags...)
	buildCmd = append(buildCmd, buildOptions()...)

	blazeArgs := append(buildCmd, targetPatterns...)

	log.Printf("running: %s %s", *buildTool, strings.Join(blazeArgs, " "))
	cmdWithStderr(*buildTool, blazeArgs...).Run()
	blazeBin := blazeInfo(config.DefaultBinDir)
	fmt.Fprintf(os.Stderr, "\n") // vertical space between build output and unused_deps output

	anyCommandPrinted := false
	for _, label := range strings.Fields(string(queryOut)) {
		_, pkg, ruleName := edit.InterpretLabel(label)
		depsByJar := directDepParams(inputFileName(blazeBin, pkg, ruleName, "jar-2.params"))
		depsToRemove := unusedDeps(inputFileName(blazeBin, pkg, ruleName, "jdeps"), depsByJar)
		// TODO(bazel-team): instead of printing, have buildifier-like modes?
		anyCommandPrinted = printCommands(label, depsToRemove) || anyCommandPrinted
	}
	if !anyCommandPrinted {
		fmt.Fprintln(os.Stderr, "No unused deps found.")
	}
}
