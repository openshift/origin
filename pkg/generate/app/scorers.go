package app

import (
	"strings"

	imageapi "github.com/openshift/origin/pkg/image/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

func templateScorer(template templateapi.Template, term string) (float32, bool) {
	score := stringProximityScorer(template.Name, term)
	return score, score < 0.3
}

func imageStreamScorer(imageStream imageapi.ImageStream, term string) (float32, bool) {
	score := stringProximityScorer(imageStream.Name, term)
	return score, score < 0.3
}

func stringProximityScorer(s, query string) float32 {
	sLower := strings.ToLower(s)
	queryLower := strings.ToLower(query)

	var score float32
	switch {
	case query == "*":
		score = 0.0
	case s == query:
		score = 0.0
	case strings.EqualFold(s, query):
		score = 0.02
	case strings.HasPrefix(s, query):
		score = 0.1
	case strings.HasPrefix(sLower, queryLower):
		score = 0.12
	case strings.Contains(s, query):
		score = 0.2
	case strings.Contains(sLower, queryLower):
		score = 0.22
	default:
		score = 1.0
	}

	return score
}

func partialScorer(a, b string, prefix bool, partial, none float32) (bool, float32) {
	switch {
	// If either one is empty, it's a partial match because the values do not conflict.
	case len(a) == 0 && len(b) != 0, len(a) != 0 && len(b) == 0:
		return true, partial
	case a != b:
		if prefix {
			if strings.HasPrefix(a, b) || strings.HasPrefix(b, a) {
				return true, partial
			}
		}
		return false, none
	default:
		return true, 0.0
	}
}
