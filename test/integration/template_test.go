// +build integration,etcd

package integration

import (
	"testing"

	"k8s.io/kubernetes/pkg/api/v1beta3"
	"k8s.io/kubernetes/pkg/runtime"

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
		config.Prefix = ""
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
				&v1beta3.Service{
					ObjectMeta: v1beta3.ObjectMeta{
						Name:      "${NAME}-tester",
						Namespace: "somevalue",
					},
					Spec: v1beta3.ServiceSpec{
						PortalIP:        "1.2.3.4",
						SessionAffinity: "some-bad-${VALUE}",
					},
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
		svc := obj.Objects[0].(*runtime.Unstructured).Object
		spec := svc["spec"].(map[string]interface{})
		meta := svc["metadata"].(map[string]interface{})
		// keep existing values
		if spec["portalIP"] != "1.2.3.4" {
			t.Fatalf("unexpected object: %#v", svc)
		}
		// replace a value
		if meta["name"] != "test-tester" {
			t.Fatalf("unexpected object: %#v", svc)
		}
		// clear namespace
		if meta["namespace"] != "" {
			t.Fatalf("unexpected object: %#v", svc)
		}
		// preserve values exactly
		if spec["sessionAffinity"] != "some-bad-${VALUE}" {
			t.Fatalf("unexpected object: %#v", svc)
		}
	}
}
