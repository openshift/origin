package imagequalify

import (
	"fmt"
	"io"
	"strings"

	"github.com/golang/glog"
	configlatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/image/admission/apis/imagequalify"
	"github.com/openshift/origin/pkg/image/admission/apis/imagequalify/validation"
)

func filterRules(rules []imagequalify.ImageQualifyRule, test func(rule *imagequalify.ImageQualifyRule) bool) []imagequalify.ImageQualifyRule {
	filtered := make([]imagequalify.ImageQualifyRule, 0, len(rules))

	for i := range rules {
		if test(&rules[i]) {
			filtered = append(filtered, rules[i])
		}
	}

	return filtered
}

func compareParts(x, y *imagequalify.ImageQualifyRule, cmp func(x, y *PatternParts) bool) bool {
	a, b := destructurePattern(x.Pattern), destructurePattern(y.Pattern)
	return cmp(&a, &b)
}

func sortRulesByPattern(rules []imagequalify.ImageQualifyRule) {
	// Comparators for sorting rules

	depth := func(x, y *imagequalify.ImageQualifyRule) bool {
		return compareParts(x, y, func(a, b *PatternParts) bool {
			return a.Depth > b.Depth
		})
	}

	digest := func(x, y *imagequalify.ImageQualifyRule) bool {
		return compareParts(x, y, func(a, b *PatternParts) bool {
			return a.Digest > b.Digest
		})
	}

	tag := func(x, y *imagequalify.ImageQualifyRule) bool {
		return compareParts(x, y, func(a, b *PatternParts) bool {
			return a.Tag > b.Tag
		})
	}

	path := func(x, y *imagequalify.ImageQualifyRule) bool {
		return compareParts(x, y, func(a, b *PatternParts) bool {
			return a.Path > b.Path
		})
	}

	explicitRules := filterRules(rules, func(rule *imagequalify.ImageQualifyRule) bool {
		return !strings.Contains(rule.Pattern, "*")
	})

	wildcardRules := filterRules(rules, func(rule *imagequalify.ImageQualifyRule) bool {
		return strings.Contains(rule.Pattern, "*")
	})

	orderBy(depth, digest, tag, path).Sort(explicitRules)
	orderBy(depth, digest, tag, path).Sort(wildcardRules)
	copy(rules, append(explicitRules, wildcardRules...))
}

func readConfig(rdr io.Reader) (*imagequalify.ImageQualifyConfig, error) {
	obj, err := configlatest.ReadYAML(rdr)
	if err != nil {
		glog.V(5).Infof("%s error reading config: %v", imagequalify.PluginName, err)
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}
	config, ok := obj.(*imagequalify.ImageQualifyConfig)
	if !ok {
		return nil, fmt.Errorf("unexpected config object: %#v", obj)
	}
	glog.V(5).Infof("%s config is: %#v", imagequalify.PluginName, config)
	if errs := validation.Validate(config); len(errs) > 0 {
		return nil, errs.ToAggregate()
	}
	if len(config.Rules) > 0 {
		sortRulesByPattern(config.Rules)
	}
	return config, nil
}
