package extended

import (
	"os"

	"k8s.io/kubernetes/test/e2e"
)

var testContext e2e.TestContextType

func kubeConfigPath() string {
	return os.Getenv("KUBECONFIG")
}
