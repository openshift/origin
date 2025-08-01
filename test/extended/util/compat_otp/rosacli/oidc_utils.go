package rosacli

import (
	"regexp"
	"strings"

	logger "github.com/openshift/origin/test/extended/util/compat_otp/logext"
)

// Split resources from the aws arn
func SplitARNResources(v string) []string {
	var parts []string
	var offset int

	for offset <= len(v) {
		idx := strings.IndexAny(v[offset:], "/:")
		if idx < 0 {
			parts = append(parts, v[offset:])
			break
		}
		parts = append(parts, v[offset:idx+offset])
		offset += idx + 1
	}
	return parts
}

// Extract the oidc provider ARN from the output of `rosa create oidc-config --mode auto` and also for common message containing the arn
func ExtractOIDCProviderARN(output string) string {
	oidcProviderArnRE := regexp.MustCompile(`arn:aws:iam::[^']+:oidc-provider/[^']+`)
	submatchall := oidcProviderArnRE.FindAllString(output, -1)
	if len(submatchall) < 1 {
		logger.Warnf("Cannot find sub string matached %s from input string %s! Please check the matching string", oidcProviderArnRE, output)
		return ""
	}
	if len(submatchall) > 1 {
		logger.Warnf("Find more than one sub string matached %s! Please check this unexpexted result then update the regex if needed.", oidcProviderArnRE)
	}
	return submatchall[0]
}

// Extract the oidc provider ARN from the output of `rosa create oidc-config --mode auto` and also for common message containing the arn
func ExtractOIDCProviderIDFromARN(arn string) string {
	spliptElements := SplitARNResources(arn)
	return spliptElements[len(spliptElements)-1]
}
