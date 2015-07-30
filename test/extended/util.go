package extended

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/GoogleCloudPlatform/kubernetes/test/e2e"
)

var testContext = e2e.TestContextType{}

func adminKubeConfigPath() string {
	if kubeConfigPath := os.Getenv("SERVER_KUBECONFIG_PATH"); len(kubeConfigPath) != 0 {
		return filepath.Join(os.Getenv("SERVER_CONFIG_DIR"), kubeConfigPath)
	}
	return filepath.Join(os.Getenv("SERVER_CONFIG_DIR"), "master", "admin.kubeconfig")
}

func writeTempJSON(path, content string) error {
	return ioutil.WriteFile(path, []byte(content), 0644)
}

func getTempFilePath(name string) string {
	return filepath.Join(testContext.OutputDir, name+".json")
}
