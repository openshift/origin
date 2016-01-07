package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	pkg_runtime "k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/golang/glog"
	flag "github.com/spf13/pflag"

	_ "github.com/openshift/origin/pkg/api"
	_ "github.com/openshift/origin/pkg/api/v1"
	_ "github.com/openshift/origin/pkg/api/v1beta3"
)

var (
	functionDest = flag.StringP("funcDest", "f", "-", "Output for conversion functions; '-' means stdout")
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

	generator := pkg_runtime.NewConversionGenerator(api.Scheme.Raw(), "github.com/openshift/origin/pkg/api")
	apiShort := generator.AddImport("k8s.io/kubernetes/pkg/api")
	generator.AddImport("k8s.io/kubernetes/pkg/api/resource")
	generator.AssumePrivateConversions()
	// TODO(wojtek-t): Change the overwrites to a flag.
	generator.OverwritePackage(*version, "")
	for _, knownType := range api.Scheme.KnownTypes(*version) {
		if !strings.Contains(knownType.PkgPath(), "openshift/origin") {
			continue
		}
		if err := generator.GenerateConversionsForType(*version, knownType); err != nil {
			glog.Errorf("error while generating conversion functions for %v: %v", knownType, err)
		}
	}

	generator.RepackImports(sets.NewString("k8s.io/kubernetes/pkg/runtime"))
	// the repack changes the name of the import
	apiShort = generator.AddImport("k8s.io/kubernetes/pkg/api")

	if err := generator.WriteImports(funcOut); err != nil {
		glog.Fatalf("error while writing imports: %v", err)
	}
	if err := generator.WriteConversionFunctions(funcOut); err != nil {
		glog.Fatalf("Error while writing conversion functions: %v", err)
	}
	if err := generator.RegisterConversionFunctions(funcOut, fmt.Sprintf("%s.Scheme", apiShort)); err != nil {
		glog.Fatalf("Error while writing conversion functions: %v", err)
	}
}
