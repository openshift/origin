// +build ignore

// How to use:
// go run make.go <flag>
//
// Flags:
// <no flag>: Build code
// -check: Run unit tests
// -test: Run full suite of tests
// -run: Run all-in-one server
// -clean: Clean up a previous build
// -release: Build official OpenShift release
// -what: Specify a target directory to build
// -tests: Specify a target directory to test
// -goflags: Specify go tool flags. Use only
// 	with equals sign eg. go run make.go -goflags=-v
//  or go run make.go -goflags='-u -v' for multiple
//  go flags

package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
)

var (
	check   = flag.Bool("check", false, "Run unit tests")
	test    = flag.Bool("test", false, "Run full suite of tests")
	run     = flag.Bool("run", false, "Run all-in-one server")
	clean   = flag.Bool("clean", false, "Clean up a previous build")
	release = flag.Bool("release", false, "Build official OpenShift release")
	what    = flag.String("what", "", "Specify a target directory to build")
	tests   = flag.String("tests", "", "Specify a target directory to test")
	goFlags = flag.String("goflags", "", "Specify go tool flags")
)

const (
	outDir    = "_output"
	outPkgDir = "Godeps/_workspace/pkg"
)

func main() {
	flag.Parse()

	envs := make([]string, 0)
	if *what != "" {
		envs = append(envs, "WHAT="+*what)
	}
	if *tests != "" {
		envs = append(envs, "TESTS="+*tests)
	}
	if *goFlags != "" {
		envs = append(envs, "GOFLAGS="+*goFlags)
	}

	// TODO: Force the use of one bool flag at a time
	switch {
	case *check:
		Check(envs)
	case *test:
		Test(envs)
	case *clean:
		Clean()
	case *release:
		Release(envs)
	case *run:
		Run(envs)
	default:
		Build(envs)
	}
}

// Build compiles OpenShift
func Build(envs []string) {
	execute("./hack/build-go.sh", envs, nil)
}

// Check builds and runs all unit tests
func Check(envs []string) {
	execute("hack/test-go.sh", envs, nil)
}

// Test builds and runs the complete test-suite
func Test(envs []string) {
	kubeRelated := []string{"KUBE_COVER= -cover -covermode=atomic", "KUBE_RACE= -race"}
	emptyKubeEnv := []string{"KUBE=\" \""}

	kubeRelated = append(kubeRelated, envs...)
	emptyKubeEnv = append(emptyKubeEnv, envs...)

	if os.Getenv("SKIP_BUILD") != "true" { // not sure about true being a string here
		Build(kubeRelated)
	} else {
		log.Println("Build is being skipped")
	}

	execute("hack/test-cmd.sh", kubeRelated, nil)
	execute("hack/test-integration.sh", emptyKubeEnv, nil)
	execute("hack/test-integration-docker.sh", emptyKubeEnv, nil)
	execute("hack/test-end-to-end.sh", kubeRelated, nil)
}

// Run runs all-in-one OpenShift server.
func Run(envs []string) {
	Build(envs)
	args := []string{"start"}
	execute("local/go/bin/openshift", nil, args)
}

// Clean removes all build artifacts
func Clean() {
	// Deliberately ignore errors returned
	// from os.Remove
	os.RemoveAll(outDir)
	os.RemoveAll(outPkgDir)
}

// Release builds an official release of OpenShift, including the official images
func Release(envs []string) {
	Clean()
	execute("hack/build-release.sh", envs, nil)
	execute("hack/build-images.sh", envs, nil)
}

func execute(command string, envs []string, args []string) {
	// TODO: Pipe process stdout to os.Stdout
	cmd := exec.Command(command, args...)
	// TODO: When passing user env variables (WHAT, GOFLAGS, TESTS) make.go crashes
	cmd.Env = envs
	if err := cmd.Run(); err != nil {
		log.Fatalln(err)
	}
}
