package errors

import (
	"runtime"

	"github.com/openshift/origin/pkg/cmd/errors"
)

const (
	KubeConfigFileSolutionWindows = `
Make sure that the value of the --config flag passed contains a valid path:
   --config=c:\path\to\valid\file
`
	KubeConfigFileSolutionUnix = `
Make sure that the value of the --config flag passed contains a valid path:
   --config=/path/to/valid/file
`
	KubeConfigSolutionUnix = `
You can unset the KUBECONFIG variable to use the default location for it:
   unset KUBECONFIG

Or you can set its value to a file that can be written to:
   export KUBECONFIG=/path/to/file
`

	KubeConfigSolutionWindows = `
You can clear the KUBECONFIG variable to use the default location for it:
   set KUBECONFIG=

Or you can set its value to a file that can be written to:
   set KUBECONFIG=c:\path\to\file`
)

// ErrKubeConfigNotWriteable is returned when the file pointed to by KUBECONFIG cannot be created or written to
// if isExplicitFile flag is true, the path to .kubeconfig was set using a --config=... flag
func ErrKubeConfigNotWriteable(file string, isExplicitFile bool, err error) error {
	return errors.NewError("KUBECONFIG is set to a file that cannot be created or modified: %s", file).WithCause(err).WithSolution(kubeConfigSolution(isExplicitFile))
}

func kubeConfigSolution(isExplicitFile bool) string {
	switch runtime.GOOS {
	case "windows":
		if isExplicitFile {
			return KubeConfigFileSolutionWindows
		}
		return KubeConfigSolutionWindows
	default:
		if isExplicitFile {
			return KubeConfigFileSolutionUnix
		}
		return KubeConfigSolutionUnix
	}
}
