package ginkgo

import (
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/funcr"
	"github.com/onsi/ginkgo/v2"
)

// this is copied from ginkgo because ginkgo made it internal and then hardcoded an init block
// using these functions to wire to os.stdout and we want to wire to stderr (or a different buffer) so we can
// have json output.

func GinkgoLogrFunc(writer ginkgo.GinkgoWriterInterface) logr.Logger {
	return funcr.New(func(prefix, args string) {
		if prefix == "" {
			writer.Printf("%s\n", args)
		} else {
			writer.Printf("%s %s\n", prefix, args)
		}
	}, funcr.Options{})
}
