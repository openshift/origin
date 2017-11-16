package clientconfig

import (
	"k8s.io/kubernetes/pkg/api"
)

func EnvVars(host string, caData []byte, insecure bool, bearerTokenFile string) []api.EnvVar {
	envvars := []api.EnvVar{
		{Name: "KUBERNETES_MASTER", Value: host},
		{Name: "OPENSHIFT_MASTER", Value: host},
	}

	if len(bearerTokenFile) > 0 {
		envvars = append(envvars, api.EnvVar{Name: "BEARER_TOKEN_FILE", Value: bearerTokenFile})
	}

	if len(caData) > 0 {
		envvars = append(envvars, api.EnvVar{Name: "OPENSHIFT_CA_DATA", Value: string(caData)})
	} else if insecure {
		envvars = append(envvars, api.EnvVar{Name: "OPENSHIFT_INSECURE", Value: "true"})
	}

	return envvars
}
