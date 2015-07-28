package describe

import (
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	ktestclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/client/testclient"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
)

func TestChainDescriber(t *testing.T) {
	tests := []struct {
		testName         string
		namespaces       kutil.StringSet
		output           string
		defaultNamespace string
		name             string
		tag              string
		path             string
		humanReadable    map[string]struct{}
		dot              []string
		expectedErr      error
	}{
		{
			testName:         "human readable test - single namespace",
			namespaces:       kutil.NewStringSet("test"),
			output:           "",
			defaultNamespace: "test",
			name:             "ruby-20-centos7",
			tag:              "latest",
			path:             "../../../../pkg/cmd/experimental/buildchain/test/single-namespace-bcs.yaml",
			humanReadable: map[string]struct{}{
				"imagestreamtag/ruby-20-centos7:latest":        {},
				"\tbc/ruby-hello-world":                        {},
				"\t\timagestreamtag/ruby-hello-world:latest":   {},
				"\tbc/ruby-sample-build":                       {},
				"\t\timagestreamtag/origin-ruby-sample:latest": {},
			},
			expectedErr: nil,
		},
		{
			testName:         "dot test - single namespace",
			namespaces:       kutil.NewStringSet("test"),
			output:           "dot",
			defaultNamespace: "test",
			name:             "ruby-20-centos7",
			tag:              "latest",
			path:             "../../../../pkg/cmd/experimental/buildchain/test/single-namespace-bcs.yaml",
			dot: []string{
				"digraph \"ruby-20-centos7:latest\" {",
				"// Node definitions.",
				"[label=\"BuildConfig|test/ruby-hello-world\"];",
				"[label=\"BuildConfig|test/ruby-sample-build\"];",
				"[label=\"ImageStreamTag|test/ruby-hello-world:latest\"];",
				"[label=\"ImageStreamTag|test/ruby-20-centos7:latest\"];",
				"[label=\"ImageStreamTag|test/origin-ruby-sample:latest\"];",
				"",
				"// Edge definitions.",
				"[label=\"BuildOutput\"];",
				"[label=\"BuildOutput\"];",
				"[label=\"BuildInputImage\"];",
				"[label=\"BuildInputImage\"];",
				"}",
			},
			expectedErr: nil,
		},
		{
			testName:         "human readable test - multiple namespaces",
			namespaces:       kutil.NewStringSet("test", "master", "default"),
			output:           "",
			defaultNamespace: "master",
			name:             "ruby-20-centos7",
			tag:              "latest",
			path:             "../../../../pkg/cmd/experimental/buildchain/test/multiple-namespaces-bcs.yaml",
			humanReadable: map[string]struct{}{
				"<master imagestreamtag/ruby-20-centos7:latest>":         {},
				"\t<default bc/ruby-hello-world>":                        {},
				"\t\t<test imagestreamtag/ruby-hello-world:latest>":      {},
				"\t<test bc/ruby-sample-build>":                          {},
				"\t\t<another imagestreamtag/origin-ruby-sample:latest>": {},
			},
			expectedErr: nil,
		},
		{
			testName:         "dot test - multiple namespaces",
			namespaces:       kutil.NewStringSet("test", "master", "default"),
			output:           "dot",
			defaultNamespace: "master",
			name:             "ruby-20-centos7",
			tag:              "latest",
			path:             "../../../../pkg/cmd/experimental/buildchain/test/multiple-namespaces-bcs.yaml",
			dot: []string{
				"digraph \"ruby-20-centos7:latest\" {",
				"// Node definitions.",
				"[label=\"BuildConfig|default/ruby-hello-world\"];",
				"[label=\"BuildConfig|test/ruby-sample-build\"];",
				"[label=\"ImageStreamTag|test/ruby-hello-world:latest\"];",
				"[label=\"ImageStreamTag|master/ruby-20-centos7:latest\"];",
				"[label=\"ImageStreamTag|another/origin-ruby-sample:latest\"];",
				"",
				"// Edge definitions.",
				"[label=\"BuildOutput\"];",
				"[label=\"BuildOutput\"];",
				"[label=\"BuildInputImage\"];",
				"[label=\"BuildInputImage\"];",
				"}",
			},
			expectedErr: nil,
		},
	}

	for _, test := range tests {
		o := ktestclient.NewObjects(kapi.Scheme, kapi.Scheme)
		if len(test.path) > 0 {
			if err := ktestclient.AddObjectsFromPath(test.path, o, kapi.Scheme); err != nil {
				t.Fatal(err)
			}
		}

		oc, _ := testclient.NewFixtureClients(o)
		ist := imagegraph.MakeImageStreamTagObjectMeta(test.defaultNamespace, test.name, test.tag)

		desc, err := NewChainDescriber(oc, test.namespaces, test.output).Describe(ist)
		if err != test.expectedErr {
			t.Fatalf("%s: error mismatch: expected %v, got %v", test.testName, test.expectedErr, err)
		}

		got := strings.Split(desc, "\n")

		switch test.output {
		case "dot":
			if len(test.dot) != len(got) {
				t.Fatalf("%s: expected %d lines, got %d", test.testName, len(test.dot), len(got))
			}
			for _, expected := range test.dot {
				if !strings.Contains(desc, expected) {
					t.Errorf("%s: unexpected description:\n%s\nexpected line in it:\n%s", test.testName, desc, expected)
				}
			}
		case "":
			if len(test.humanReadable) != len(got) {
				t.Fatalf("%s: expected %d lines, got %d", test.testName, len(test.humanReadable), len(got))
			}
			for _, line := range got {
				if _, ok := test.humanReadable[line]; !ok {
					t.Errorf("%s: unexpected line: %s", test.testName, line)
				}
			}
		}
	}
}
