package helpers

import "os"

// EnvFallBack ...
func EnvFallBack(envs []string, defaultValue string) string {
	for _, k := range envs {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return defaultValue
}

// GetPowerEndPoint
func GetPowerEndPoint() string {
	return EnvFallBack([]string{"IBMCLOUD_POWER_API_ENDPOINT"}, "")
}
