package imagequalify

import (
	"path"

	"github.com/openshift/origin/pkg/image/admission/apis/imagequalify"
	"github.com/openshift/origin/pkg/image/admission/apis/imagequalify/validation"
)

// matchImage attempts to match image against each pattern defined in
// rules. Returns the Rule that matched, or nil if no match was found.
func matchImage(image string, rules []imagequalify.ImageQualifyRule) *imagequalify.ImageQualifyRule {
	for i := range rules {
		if ok, _ := path.Match(rules[i].Pattern, image); ok {
			return &rules[i]
		}
	}
	return nil
}

func qualifyImage(image string, rules []imagequalify.ImageQualifyRule) (string, error) {
	domain, _, err := validation.ParseDomainName(image)
	if err != nil {
		return "", err
	}

	if domain != "" {
		// image is already qualified
		return image, nil
	}

	matched := matchImage(image, rules)
	if matched == nil {
		// no match, return as-is.
		return image, nil
	}

	qname := matched.Domain + "/" + image

	// Revalidate qualified image
	if _, _, err := validation.ParseDomainName(qname); err != nil {
		return "", err
	}

	return qname, nil
}
