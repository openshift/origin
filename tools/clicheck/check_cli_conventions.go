package main

import (
	"fmt"
	"os"

	"github.com/openshift/origin/pkg/cmd/openshift"
	cmdsanity "github.com/openshift/origin/pkg/cmd/util/sanity"
)

var (
	skip = []string{
		"openshift kube",             // TODO enable when we upstream all these conventions
		"openshift start kubernetes", // TODO enable when we upstream all these conventions
		"openshift cli create quota", // TODO has examples starting with '//', enable when we upstream all these conventions
		"openshift cli adm",          // already checked in 'openshift admin'
		"openshift ex",               // we will only care about experimental when they get promoted
		"openshift cli types",
	}
)

func main() {
	errors := []error{}

	oc := openshift.NewCommandOpenShift("openshift")
	result := cmdsanity.CheckCmdTree(oc, cmdsanity.AllCmdChecks, skip)
	errors = append(errors, result...)

	if len(errors) > 0 {
		for i, err := range errors {
			fmt.Fprintf(os.Stderr, "%d. %s\n\n", i+1, err)
		}
		os.Exit(1)
	}

	fmt.Fprintln(os.Stdout, "Congrats, CLI looks good!")
}
