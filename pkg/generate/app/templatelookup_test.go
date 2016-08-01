package app

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/client/testclient"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

func testTemplateClient(templates *templateapi.TemplateList) client.Interface {
	fake := &testclient.Fake{}
	fake.AddReactor("list", "templates", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		if len(action.GetNamespace()) > 0 {
			matchingTemplates := &templateapi.TemplateList{
				Items: []templateapi.Template{},
			}
			for _, template := range templates.Items {
				if template.Namespace == action.GetNamespace() {
					matchingTemplates.Items = append(matchingTemplates.Items, template)
				}
			}
			return true, matchingTemplates, nil
		} else {
			return true, templates, nil
		}
	})
	return fake
}

func TestTemplateSearcher(t *testing.T) {
	testData := map[string][]string{
		"openshift":   {"nodejs-mongodb-example", "rails-postgresql-example", "jenkins-ephemeral", "my-jenkins"},
		"mynamespace": {"my-jenkins"},
	}

	tests := []struct {
		value           string
		expectedMatches int
		expectedErr     bool
	}{
		{
			value:           "jenkins",
			expectedMatches: 3,
		},
		{
			value:           "my-jenkins",
			expectedMatches: 2,
		},
		{
			value:           "jenkins-ephemeral",
			expectedMatches: 1,
		},
		{
			value:           "openshift/my-jenkins",
			expectedMatches: 1,
		},
		{
			value:           "openshift/jenkins",
			expectedMatches: 2,
		},
		{
			value:           "foobar",
			expectedMatches: 0,
		},
		{
			value:           "openshift/foobar",
			expectedMatches: 0,
		},
		{
			value:           "mynamespace/jenkins-ephemeral",
			expectedMatches: 0,
		},
		{
			value:           "mynamespace/jenkins-ephemeral",
			expectedMatches: 0,
		},
		{
			value:       "foo/bar/zee",
			expectedErr: true,
		},
	}

	templates := fakeTemplates(testData)
	client := testTemplateClient(templates)
	searcher := &TemplateSearcher{
		Client:     client,
		Namespaces: []string{"openshift", "mynamespace"},
	}
	for _, test := range tests {
		searchResults, errs := searcher.Search(false, test.value)
		if errs != nil && !test.expectedErr {
			t.Errorf("unexpected errors: %v", errs)
		}
		if len(searchResults) != test.expectedMatches {
			t.Errorf("Expected %v search matches for %q, got %v", test.expectedMatches, test.value, len(searchResults))
		}
	}
}

func fakeTemplates(testData map[string][]string) *templateapi.TemplateList {
	templates := &templateapi.TemplateList{
		Items: []templateapi.Template{},
	}
	for namespace, templateNames := range testData {
		for _, templateName := range templateNames {
			template := &templateapi.Template{
				ObjectMeta: kapi.ObjectMeta{
					Name:      templateName,
					Namespace: namespace,
				},
			}
			templates.Items = append(templates.Items, *template)
		}
	}
	return templates
}
