package mesos_test

import (
	"testing"

	. "github.com/mesos/mesos-go/api/v1/lib/resourcetest"
)

func BenchmarkPrecisionScalarMath(b *testing.B) {
	var (
		start   = Resources(Resource(Name("cpus"), ValueScalar(1.001)))
		current = start.Clone()
	)
	for i := 0; i < b.N; i++ {
		current = current.Plus(current...).Plus(current...).Minus(current...).Minus(current...)
	}
}
