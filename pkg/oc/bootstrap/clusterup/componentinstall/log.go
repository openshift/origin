package componentinstall

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func LogContainer(logdir, name, stdout, stderr string) error {
	if err := os.MkdirAll(logdir, 0755); err != nil {
		return err
	}

	spinNumber := 1
	stdoutFile := ""
	stderrFile := ""
	for i := 0; i < 1000; i++ {
		stdoutFile = fmt.Sprintf("%s-%03d.stdout", strings.Replace(name, "/", "-", -1), spinNumber)
		stderrFile = fmt.Sprintf("%s-%03d.stderr", strings.Replace(name, "/", "-", -1), spinNumber)

		if _, err := os.Stat(path.Join(logdir, stdoutFile)); !os.IsNotExist(err) {
			continue
		}
		if _, err := os.Stat(path.Join(logdir, stderrFile)); !os.IsNotExist(err) {
			continue
		}
	}

	stdoutErr := ioutil.WriteFile(path.Join(logdir, stdoutFile), []byte(stdout), 0644)
	stderrErr := ioutil.WriteFile(path.Join(logdir, stderrFile), []byte(stderr), 0644)

	switch {
	case stdoutErr == nil && stderrErr == nil:
		return nil
	case stdoutErr != nil && stderrErr == nil:
		return stdoutErr
	case stdoutErr == nil && stderrErr != nil:
		return stderrErr
	default:
		return utilerrors.NewAggregate([]error{stdoutErr, stderrErr})
	}
}
