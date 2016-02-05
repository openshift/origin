/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	pkg_runtime "k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"

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
	functionDest = flag.StringP("func-dest", "f", "-", "Output for deep copy functions; '-' means stdout")
	group        = flag.StringP("group", "g", "", "Group for deep copies.")
	version      = flag.StringP("version", "v", "v1beta3", "Version for deep copies.")
	overwrites   = flag.StringP("overwrites", "o", "", "Comma-separated overwrites for package names")
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

	knownGroupVersion := unversioned.GroupVersion{Group: *group, Version: *version}
	if knownGroupVersion.Version == "api" {
		knownGroupVersion.Version = pkg_runtime.APIVersionInternal
	}
	generator := pkg_runtime.NewDeepCopyGenerator(api.Scheme.Raw(), "github.com/openshift/origin/pkg/api", sets.NewString("github.com/openshift/origin"))
	apiShort := generator.AddImport("k8s.io/kubernetes/pkg/api")
	generator.ReplaceType("k8s.io/kubernetes/pkg/util/sets", "empty", struct{}{})

	for _, overwrite := range strings.Split(*overwrites, ",") {
		vals := strings.Split(overwrite, "=")
		generator.OverwritePackage(vals[0], vals[1])
	}
	for _, knownType := range api.Scheme.KnownTypes(knownGroupVersion) {
		if !strings.Contains(knownType.PkgPath(), "openshift/origin") {
			continue
		}
		if err := generator.AddType(knownType); err != nil {
			glog.Errorf("error while generating deep copy functions for %v: %v", knownType, err)
		}
	}

	generator.RepackImports()
	// the repack changes the name of the import
	apiShort = generator.AddImport("k8s.io/kubernetes/pkg/api")

	if err := generator.WriteImports(funcOut); err != nil {
		glog.Fatalf("error while writing imports: %v", err)
	}
	if err := generator.WriteDeepCopyFunctions(funcOut); err != nil {
		glog.Fatalf("error while writing deep copy functions: %v", err)
	}
	if err := generator.RegisterDeepCopyFunctions(funcOut, fmt.Sprintf("%s.Scheme", apiShort)); err != nil {
		glog.Fatalf("error while registering deep copy functions: %v", err)
	}
}
