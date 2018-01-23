package imagequalify

import (
	"fmt"
	"io"
	"strings"

	"github.com/golang/glog"
	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/image/admission/imagequalify/api"
	"github.com/openshift/origin/pkg/image/admission/imagequalify/api/validation"
)

func filterRules(rules []api.ImageQualifyRule, test func(rule *api.ImageQualifyRule) bool) []api.ImageQualifyRule {
	filtered := make([]api.ImageQualifyRule, 0, len(rules))

	for i := range rules {
		if test(&rules[i]) {
			filtered = append(filtered, rules[i])
		}
	}

	return filtered
}

func compareParts(x, y *api.ImageQualifyRule, cmp func(x, y *PatternParts) bool) bool {
	a, b := destructurePattern(x.Pattern), destructurePattern(y.Pattern)
	return cmp(&a, &b)
}

func sortRulesByPattern(rules []api.ImageQualifyRule) {
	// Comparators for sorting rules

	depth := func(x, y *api.ImageQualifyRule) bool {
		return compareParts(x, y, func(a, b *PatternParts) bool {
			return a.Depth > b.Depth
		})
	}

	digest := func(x, y *api.ImageQualifyRule) bool {
		return compareParts(x, y, func(a, b *PatternParts) bool {
			return a.Digest > b.Digest
		})
	}

	tag := func(x, y *api.ImageQualifyRule) bool {
		return compareParts(x, y, func(a, b *PatternParts) bool {
			return a.Tag > b.Tag
		})
	}

	path := func(x, y *api.ImageQualifyRule) bool {
		return compareParts(x, y, func(a, b *PatternParts) bool {
			return a.Path > b.Path
		})
	}

	explicitRules := filterRules(rules, func(rule *api.ImageQualifyRule) bool {
		return !strings.Contains(rule.Pattern, "*")
	})

	wildcardRules := filterRules(rules, func(rule *api.ImageQualifyRule) bool {
		return strings.Contains(rule.Pattern, "*")
	})

	orderBy(depth, digest, tag, path).Sort(explicitRules)
	orderBy(depth, digest, tag, path).Sort(wildcardRules)
	copy(rules, append(explicitRules, wildcardRules...))
}

func readConfig(rdr io.Reader) (*api.ImageQualifyConfig, error) {
	obj, err := configlatest.ReadYAML(rdr)
	if err != nil {
		glog.V(5).Infof("%s error reading config: %v", api.PluginName, err)
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}
	config, ok := obj.(*api.ImageQualifyConfig)
	if !ok {
		return nil, fmt.Errorf("unexpected config object: %#v", obj)
	}
	glog.V(5).Infof("%s config is: %#v", api.PluginName, config)
	if errs := validation.Validate(config); len(errs) > 0 {
		return nil, errs.ToAggregate()
	}
	if len(config.Rules) > 0 {
		sortRulesByPattern(config.Rules)
	}
	return config, nil
}
