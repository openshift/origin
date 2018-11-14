package ginkgo

import (
	"github.com/onsi/ginkgo/internal/suite"
	"github.com/onsi/ginkgo/internal/writer"
)

func GlobalSuite() *suite.Suite {
	return globalSuite
}

func GinkgoWriterType() *writer.Writer {
	return GinkgoWriter.(*writer.Writer)
}
