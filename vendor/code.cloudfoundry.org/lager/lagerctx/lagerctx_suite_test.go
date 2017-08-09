package lagerctx_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLagerctx(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lagerctx Suite")
}
