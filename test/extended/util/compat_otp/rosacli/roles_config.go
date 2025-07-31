package rosacli

import (
	"os"
	"strings"
)

// Get installer role arn from ${SHARED_DIR}/account-roles-arns
func GetInstallerRoleArn(hostedcp bool) (string, error) {
	sharedDIR := os.Getenv("SHARED_DIR")
	filePath := sharedDIR + "/account-roles-arns"
	fileContents, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(fileContents), "\n")
	for i := range lines {
		if hostedcp && strings.Contains(lines[i], "-HCP-ROSA-Installer-Role") {
			return lines[i], nil
		}
		if !hostedcp && !strings.Contains(lines[i], "-ROSA-Installer-Role") && strings.Contains(lines[i], "-Installer-Role") {
			return lines[i], nil
		}
		continue
	}
	return "", nil
}
