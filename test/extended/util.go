package extended

import (
	"os"

	"github.com/GoogleCloudPlatform/kubernetes/test/e2e"
)

var testContext e2e.TestContextType

func kubeConfigPath() string {
	return os.Getenv("KUBECONFIG")
}
