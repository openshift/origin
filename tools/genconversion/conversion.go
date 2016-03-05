package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"sort"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	pkg_runtime "k8s.io/kubernetes/pkg/runtime"

	"github.com/golang/glog"
	flag "github.com/spf13/pflag"

	_ "github.com/openshift/origin/pkg/api"
	_ "github.com/openshift/origin/pkg/api/v1"
	_ "github.com/openshift/origin/pkg/api/v1beta3"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"
)

var (
	functionDest = flag.StringP("funcDest", "f", "-", "Output for conversion functions; '-' means stdout")
	group        = flag.StringP("group", "g", "", "Group for conversion.")
	version      = flag.StringP("version", "v", "v1beta3", "Version for conversion.")
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.Parse()
	log.SetOutput(os.Stderr)

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

	generator := pkg_runtime.NewConversionGenerator(api.Scheme, "github.com/openshift/origin/pkg/api")
	apiShort := generator.AddImport("k8s.io/kubernetes/pkg/api")
	generator.AddImport("k8s.io/kubernetes/pkg/api/resource")
	generator.AssumePrivateConversions()
	// TODO(wojtek-t): Change the overwrites to a flag.
	generator.OverwritePackage(*version, "")
	gv := unversioned.GroupVersion{Group: *group, Version: *version}

	knownTypes := api.Scheme.KnownTypes(gv)
	knownTypeKeys := []string{}
	for key := range knownTypes {
		knownTypeKeys = append(knownTypeKeys, key)
	}
	sort.Strings(knownTypeKeys)

	for _, knownTypeKey := range knownTypeKeys {
		knownType := knownTypes[knownTypeKey]
		if !strings.Contains(knownType.PkgPath(), "openshift/origin") {
			continue
		}
		if err := generator.GenerateConversionsForType(gv, knownType); err != nil {
			glog.Errorf("error while generating conversion functions for %v: %v", knownType, err)
		}
	}

	// generator.RepackImports(sets.NewString("k8s.io/kubernetes/pkg/runtime"))
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
