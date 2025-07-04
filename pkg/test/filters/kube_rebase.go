package filters

import (
	"context"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/test/extensions"
)

// KubeRebaseTestsFilter filters out tests during k8s rebase
type KubeRebaseTestsFilter struct {
	restConfig *rest.Config
}

func NewKubeRebaseTestsFilter(restConfig *rest.Config) *KubeRebaseTestsFilter {
	return &KubeRebaseTestsFilter{
		restConfig: restConfig,
	}
}

func (f *KubeRebaseTestsFilter) Name() string {
	return "kube-rebase-tests"
}

func (f *KubeRebaseTestsFilter) Filter(ctx context.Context, tests extensions.ExtensionTestSpecs) (extensions.ExtensionTestSpecs, error) {
	if f.restConfig == nil {
		return tests, nil
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(f.restConfig)
	if err != nil {
		return nil, err
	}
	serverVersion, err := discoveryClient.ServerVersion()
	if err != nil {
		return nil, err
	}

	// TODO: this version along with below exclusions lists needs to be updated
	// for the rebase in-progress.
	if !strings.HasPrefix(serverVersion.Minor, "31") {
		return tests, nil
	}

	// Below list should only be filled in when we're trying to land k8s rebase.
	// Don't pile them up!
	exclusions := []string{
		// affected by the available controller split https://github.com/kubernetes/kubernetes/pull/126149
		`[sig-api-machinery] health handlers should contain necessary checks`,
	}

	matches := make(extensions.ExtensionTestSpecs, 0, len(tests))
outerLoop:
	for _, test := range tests {
		for _, excl := range exclusions {
			if strings.Contains(test.Name, excl) {
				logrus.Infof("Skipping %q due to rebase in-progress", test.Name)
				continue outerLoop
			}
		}
		matches = append(matches, test)
	}
	return matches, nil
}

func (f *KubeRebaseTestsFilter) ShouldApply() bool {
	return f.restConfig != nil
}
