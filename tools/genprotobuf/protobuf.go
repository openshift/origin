// go-to-protobuf generates a Protobuf IDL from a Go struct, respecting any
// existing IDL tags on the Go struct.
package main

import (
	"path/filepath"
	"strings"

	"k8s.io/gengo/args"
	"k8s.io/kubernetes/cmd/libs/go2idl/go-to-protobuf/protobuf"

	flag "github.com/spf13/pflag"
)

var g = protobuf.New()

func init() {
	sourceTree := args.DefaultSourceTree()
	g.Common.GoHeaderFilePath = filepath.Join("hack", "boilerplate.txt")
	g.ProtoImport = []string{
		filepath.Join("vendor"),
		filepath.Join("vendor", "k8s.io", "kubernetes", "third_party", "protobuf"),
	}
	g.OutputBase = sourceTree

	var fullPackageList []string

	if len(g.Packages) > 0 {
		// start with the predefined package list from kube's command
		kubePackages := strings.Split(g.Packages, ",")
		fullPackageList = make([]string, 0, len(kubePackages))
		for _, kubePackage := range kubePackages {
			// strip off the leading + if it exists because we want all kube packages to be prefixed with -
			// so they're not generated
			if strings.HasPrefix(kubePackage, "+") {
				kubePackage = kubePackage[1:]
			}
			fullPackageList = append(fullPackageList, "-"+kubePackage)
		}
	}

	// add the origin packages
	fullPackageList = append(fullPackageList,
		`github.com/openshift/origin/pkg/authorization/api/v1`,
		`github.com/openshift/origin/pkg/build/api/v1`,
		`github.com/openshift/origin/pkg/deploy/api/v1`,
		`github.com/openshift/origin/pkg/image/api/v1`,
		`github.com/openshift/origin/pkg/oauth/api/v1`,
		`github.com/openshift/origin/pkg/project/api/v1`,
		`github.com/openshift/origin/pkg/quota/api/v1`,
		`github.com/openshift/origin/pkg/route/api/v1`,
		`github.com/openshift/origin/pkg/sdn/api/v1`,
		`github.com/openshift/origin/pkg/security/api/v1`,
		`github.com/openshift/origin/pkg/template/api/v1`,
		`github.com/openshift/origin/pkg/user/api/v1`,
	)
	g.Packages = strings.Join(fullPackageList, ",")

	g.BindFlags(flag.CommandLine)
}

func main() {
	flag.Parse()
	protobuf.Run(g)
}
