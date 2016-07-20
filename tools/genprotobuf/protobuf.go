// go-to-protobuf generates a Protobuf IDL from a Go struct, respecting any
// existing IDL tags on the Go struct.
package main

import (
	"path/filepath"
	"strings"

	"k8s.io/kubernetes/cmd/libs/go2idl/args"
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
	g.Packages = strings.Join([]string{
		`-k8s.io/kubernetes/pkg/util/intstr`,
		`-k8s.io/kubernetes/pkg/api/resource`,
		`-k8s.io/kubernetes/pkg/runtime`,
		`-k8s.io/kubernetes/pkg/watch/versioned`,
		`-k8s.io/kubernetes/pkg/api/unversioned`,
		`-k8s.io/kubernetes/pkg/api/v1`,
		`-k8s.io/kubernetes/pkg/apis/policy/v1alpha1`,
		`-k8s.io/kubernetes/pkg/apis/extensions/v1beta1`,
		`-k8s.io/kubernetes/pkg/apis/autoscaling/v1`,
		`-k8s.io/kubernetes/pkg/apis/batch/v1`,
		`-k8s.io/kubernetes/pkg/apis/batch/v2alpha1`,
		`-k8s.io/kubernetes/pkg/apis/apps/v1alpha1`,
		`-k8s.io/kubernetes/federation/apis/federation/v1beta1`,

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
	}, ",")

	g.BindFlags(flag.CommandLine)
}

func main() {
	flag.Parse()
	protobuf.Run(g)
}
