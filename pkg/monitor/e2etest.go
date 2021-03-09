package monitor

import (
	"fmt"
)

func E2ETestLocator(testName string) string {
	return fmt.Sprintf("e2e-test/%q", testName)
}
