package lager_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestLager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lager Suite")
}
