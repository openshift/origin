package ginkgo

import (
	"time"

	"github.com/onsi/ginkgo/v2/internal"
	"github.com/onsi/ginkgo/v2/internal/global"
	"github.com/onsi/ginkgo/v2/internal/interrupt_handler"
	"github.com/onsi/ginkgo/v2/internal/parallel_support"
	"github.com/onsi/ginkgo/v2/types"
)

func SetReporterConfig(r types.ReporterConfig) {
	reporterConfig = r
}

func AppendSpecText(test *internal.Spec, text string) {
	test.AppendText(text)
}

func GetSuite() *internal.Suite {
	return global.Suite
}

func GetSpecs() internal.Specs {
	tree := global.Suite.GetTree()
	specs := internal.GenerateSpecsFromTreeRoot(tree)
	return specs
}

type Spec struct {
	InternalSpec internal.Spec
}

func GetFailer() *internal.Failer {
	return global.Failer
}

func GetWriter() *internal.Writer {
	return GinkgoWriter.(*internal.Writer)
}

func GetOutputInterceptor() internal.NoopOutputInterceptor {
	return internal.NoopOutputInterceptor{}
}

func NewInterruptHandler(timeout time.Duration, client parallel_support.Client) *interrupt_handler.InterruptHandler {
	return interrupt_handler.NewInterruptHandler(timeout, client)
}
