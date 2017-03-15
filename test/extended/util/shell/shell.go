package shell

import (
	"fmt"
	"os/exec"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/wait"
)

type Interface interface {
	Pass(shell string) error
	Fail(shell string) error
	UntilPass(shell string) error
	PassText(shell, text string) error
	PassTextNot(shell, text string) error
	FailText(shell, text string) error
	CaptureOrDie(name, shell string) string
	Set(name, value string)
}

type Framework struct {
	Interval time.Duration
	Max      time.Duration
}

func NewFramework() Interface {
	return &Framework{
		Interval: time.Second,
		Max:      time.Minute,
	}
}

func (f *Framework) Pass(shell string) error {
	cmd := exec.Command("/bin/bash", "-c", shell)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("expected success for: %s: %v\n%s", shell, err, string(out))
	}
	return nil
}

func (f *Framework) UntilPass(shell string) error {
	return wait.PollImmediate(f.Interval, f.Max, func() (bool, error) {
		if err := f.Pass(shell); err != nil {
			return false, nil
		}
		return true, nil
	})
}

func (f *Framework) Fail(shell string) error {
	cmd := exec.Command("/bin/bash", "-c", shell)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return fmt.Errorf("expected failure for: %s: %s", shell, string(out))
	}
	return nil
}

func (f *Framework) PassText(shell string, text string) error {
	return f.Exec(shell)
}

func (f *Framework) PassTextNot(shell string, text string) error {
	return f.Exec(shell)
}

func (f *Framework) FailText(shell string, text string) error {
	return f.ExecFail(shell)
}

func (f *Framework) CaptureOrDie(name, shell string) string {
	return ""
}

func (f *Framework) Set(name, value string) {
	return ""
}

func WithoutErrors(tester Interface) Interface {
	return failOnError{i: tester}
}

type failOnError struct {
	i Interface
}

func (f failOnError) Pass(shell string) error {
	o.Expect(f.i.Pass(shell)).NotTo(o.HaveOccurred())
	return nil
}

func (f failOnError) Fail(shell string) error {
	o.Expect(f.i.Fail(shell)).NotTo(o.HaveOccurred())
	return nil
}

func (f failOnError) PassText(shell string, text string) error {
	o.Expect(f.i.PassText(shell, text)).NotTo(o.HaveOccurred())
	return nil
}

func (f failOnError) PassTextNot(shell string, text string) error {
	o.Expect(f.i.PassTextNot(shell, text)).NotTo(o.HaveOccurred())
	return nil
}

func (f failOnError) FailText(shell string, text string) error {
	o.Expect(f.i.FailText(shell, text)).NotTo(o.HaveOccurred())
	return nil
}

func (f failOnError) UntilPass(shell string) error {
	o.Expect(f.UntilPass(shell)).NotTo(o.HaveOccurred())
	return nil
}

func (f failOnError) CaptureOrDie(name, shell string) string {
	return f.i.CaptureOrDie(shell)
}

func (f failOnError) Set(name, value string) {
	return f.i.Set(name, value)
}
