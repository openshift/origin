package builder

import (
	"bufio"
	"io"
	"os"
	"strings"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// getBuildEnvVars returns a map with the environment variables that should be added
// to the built image
func getBuildEnvVars(build *buildapi.Build) map[string]string {
	envVars := map[string]string{
		"OPENSHIFT_BUILD_NAME":      build.Name,
		"OPENSHIFT_BUILD_NAMESPACE": build.Namespace,
		"OPENSHIFT_BUILD_SOURCE":    build.Parameters.Source.Git.URI,
	}
	if build.Parameters.Source.Git.Ref != "" {
		envVars["OPENSHIFT_BUILD_REFERENCE"] = build.Parameters.Source.Git.Ref
	}
	if build.Parameters.Revision != nil &&
		build.Parameters.Revision.Git != nil &&
		build.Parameters.Revision.Git.Commit != "" {
		envVars["OPENSHIFT_BUILD_COMMIT"] = build.Parameters.Revision.Git.Commit
	}
	if build.Parameters.Strategy.Type == buildapi.SourceBuildStrategyType {
		userEnv := build.Parameters.Strategy.SourceStrategy.Env
		for _, v := range userEnv {
			envVars[v.Name] = v.Value
		}
	}
	return envVars
}

const dnsConfig = "/etc/resolv.conf"

func getDNSConfig() (dns []string, dnsSearch []string) {
	f, err := os.Open(dnsConfig)
	if err != nil {
		return nil, nil
	}
	defer f.Close()
	return readDNSConfig(f)
}

func readDNSConfig(r io.Reader) (dns []string, dnsSearch []string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		switch fields[0] {
		case "nameserver":
			dns = append(dns, fields[1])
		case "search":
			for i := 1; i < len(fields); i++ {
				dnsSearch = append(dnsSearch, fields[i])
			}
		}
	}
	return
}
