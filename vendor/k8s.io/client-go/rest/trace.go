package rest

import (
	"context"
	"strings"
	"fmt"
	"time"
	"io"

	"github.com/onsi/ginkgo/v2"
)

const (
	describeKey  = "describe"
	testNameKey  = "testName"
	stepNamesKey = "stepNames"
)

func WithDescription(parent context.Context, description string) context.Context {
	return context.WithValue(parent, describeKey, description)
}

func DescriptionFrom(ctx context.Context) (string, bool) {
	description, ok := ctx.Value(describeKey).(string)
	return description, ok
}

func WithTestName(parent context.Context, testName string) context.Context {
	return context.WithValue(parent, testNameKey, testName)
}

func TestNameFrom(ctx context.Context) (string, bool) {
	testName, ok := ctx.Value(testNameKey).(string)
	return testName, ok
}

func AddStep(parent context.Context, stepName string) context.Context {
	stepNamesToSet := []string{}
	if currSteps, ok := StepsFrom(parent); ok {
		stepNamesToSet = append(stepNamesToSet, currSteps...)
	}
	stepNamesToSet = append(stepNamesToSet, stepName)
	return context.WithValue(parent, stepNamesKey, stepNamesToSet)
}

func StepsFrom(ctx context.Context) ([]string, bool) {
	stepNames, ok := ctx.Value(stepNamesKey).([]string)
	return stepNames, ok
}

func StepEnd(ctx context.Context, startTime time.Time) {
	description, _ := DescriptionFrom(ctx)
	testName, _ := TestNameFrom(ctx)
	endTime := time.Now()
	steps, _ := StepsFrom(ctx)

	level:="INFO"
	format := "TEST TIMING: %q -- for step (%v) -- %v"
	fmt.Fprintf(ginkgo.GinkgoWriter, nowStamp()+": "+level+": "+format+"\n", description+" "+testName, strings.Join(steps, "."), endTime.Sub(startTime))
}

func nowStamp() string {
	return time.Now().Format(time.StampMilli)
}

func GinkgoLogf(format string, args ...interface{}){
	level:="INFO"
	fmt.Fprintf(ginkgo.GinkgoWriter, nowStamp()+": "+level+": "+format+"\n", args...)
}

func ReadAll(ctx context.Context, r io.Reader) ([]byte, error) {
	b := make([]byte, 0, 512)
	count := 0
	for {
		count++
		if len(b) == cap(b) {
			// Add more capacity (let append pick how much).
			b = append(b, 0)[:len(b)]
		}
		n, err := r.Read(b[len(b):cap(b)])
		b = b[:len(b)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}

			GinkgoLogf("reading took %v reads", count)
			return b, err
		}
	}
}