// +build integration,!no-etcd

package integration

import (
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta1"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
	templateapi "github.com/openshift/origin/pkg/template/api"
	testutil "github.com/openshift/origin/test/util"
)

func init() {
	testutil.RequireEtcd()
}
func TestTemplate(t *testing.T) {
	_, path, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, version := range []string{"v1", "v1beta3"} {
		config, err := testutil.GetClusterAdminClientConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		config.Version = version
		c, err := client.New(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		template := &templateapi.Template{
			Parameters: []templateapi.Parameter{
				{
					Name:  "NAME",
					Value: "test",
				},
			},
			Objects: []runtime.Object{
				&v1beta1.Service{
					TypeMeta: v1beta1.TypeMeta{
						ID:        "${NAME}-tester",
						Namespace: "somevalue",
					},
					PortalIP:        "1.2.3.4",
					SessionAffinity: "some-bad-${VALUE}",
				},
			},
		}

		obj, err := c.TemplateConfigs("default").Create(template)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(obj.Objects) != 1 {
			t.Fatalf("unexpected object: %#v", obj)
		}
		if err := runtime.DecodeList(obj.Objects, runtime.UnstructuredJSONScheme); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// keep existing values
		if obj.Objects[0].(*runtime.Unstructured).Object["portalIP"] != "1.2.3.4" {
			t.Fatalf("unexpected object: %#v", obj)
		}
		// replace a value
		if obj.Objects[0].(*runtime.Unstructured).Object["id"] != "test-tester" {
			t.Fatalf("unexpected object: %#v", obj)
		}
		// clear namespace
		if obj.Objects[0].(*runtime.Unstructured).Object["namespace"] != "" {
			t.Fatalf("unexpected object: %#v", obj)
		}
		// preserve values exactly
		if obj.Objects[0].(*runtime.Unstructured).Object["sessionAffinity"] != "some-bad-${VALUE}" {
			t.Fatalf("unexpected object: %#v", obj)
		}
	}
}
