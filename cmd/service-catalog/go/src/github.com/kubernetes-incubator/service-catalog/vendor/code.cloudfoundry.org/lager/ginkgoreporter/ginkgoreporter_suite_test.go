package ginkgoreporter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGinkgoReporter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GinkgoReporter Suite")
}
