package main

import (
	"github.com/openshift/origin/pkg/cmd/infra/gitserver"
	"github.com/openshift/origin/pkg/cmd/util/standard"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/api/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"
)

func main() {
	standard.Run(gitserver.CommandFor)
}
