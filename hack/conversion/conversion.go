package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/golang/glog"
	flag "github.com/spf13/pflag"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	_ "github.com/openshift/origin/pkg/api/latest"
)

var (
	functionDest = flag.StringP("funcDest", "f", "-", "Output for conversion functions; '-' means stdout")
	namesDest    = flag.StringP("nameDest", "n", "-", "Output for function names; '-' means stdout")
	version      = flag.StringP("version", "v", "v1beta3", "Version for conversion.")
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.Parse()

	var funcOut io.Writer
	if *functionDest == "-" {
		funcOut = os.Stdout
	} else {
		file, err := os.Create(*functionDest)
		if err != nil {
			glog.Fatalf("Couldn't open %v: %v", *functionDest, err)
		}
		defer file.Close()
		funcOut = file
	}
	var nameOut io.Writer
	if *namesDest == "-" {
		nameOut = os.Stdout
	} else {
		file, err := os.Create(*namesDest)
		if err != nil {
			glog.Fatalf("Couldn't open %v: %v", *functionDest, err)
		}
		defer file.Close()
		nameOut = file
	}

	generator := conversion.NewGenerator(kapi.Scheme.Raw())
	// TODO(wojtek-t): Change the overwrites to a flag.
	generator.OverwritePackage(*version, "")
	generator.OverwritePackage("api", "newer")
	for _, knownType := range kapi.Scheme.KnownTypes(*version) {
		if strings.Contains(knownType.PkgPath(), "GoogleCloudPlatform/kubernetes") {
			continue
		}
		if err := generator.GenerateConversionsForType(*version, knownType); err != nil {
			util.HandleError(fmt.Errorf("error while generating conversion functions for %v: %v", knownType, err))
		}
	}
	if err := generator.WriteConversionFunctions(funcOut); err != nil {
		glog.Fatalf("Error while writing conversion functions: %v", err)
	}
	if err := generator.WriteConversionFunctionNames(nameOut); err != nil {
		glog.Fatalf("Error while writing conversion functions: %v", err)
	}
}
