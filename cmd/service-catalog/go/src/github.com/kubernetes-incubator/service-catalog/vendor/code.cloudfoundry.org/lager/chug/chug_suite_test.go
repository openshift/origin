package chug_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestChug(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Chug Suite")
}
