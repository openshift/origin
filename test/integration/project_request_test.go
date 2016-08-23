package integration

import (
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	projectapi "github.com/openshift/origin/pkg/project/api"
	"github.com/openshift/origin/pkg/project/registry/projectrequest/delegated"
	templateapi "github.com/openshift/origin/pkg/template/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestProjectRequestError(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)

	const (
		ns                = "testns"
		templateNamespace = "default"
		templateName      = "project-request-template"
	)
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}

	masterConfig.ProjectConfig.ProjectRequestTemplate = templateNamespace + "/" + templateName

	kubeConfigFile, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	kubeClient, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %v", err)
	}
	openshiftClient, err := testutil.GetClusterAdminClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting openshift client: %v", err)
	}

	// Create custom template
	template := delegated.DefaultTemplate()
	template.Name = templateName

	additionalObjects := []runtime.Object{
		// Append an object that will succeed
		&kapi.ConfigMap{ObjectMeta: kapi.ObjectMeta{Name: "configmapname"}},
		// Append a custom object that will fail validation
		&kapi.ConfigMap{},
		// Append another object that should never be created, since we short circuit
		&kapi.ConfigMap{ObjectMeta: kapi.ObjectMeta{Name: "configmapname2"}},
	}
	if err := templateapi.AddObjectsToTemplate(template, additionalObjects, kapiv1.SchemeGroupVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := openshiftClient.Templates(templateNamespace).Create(template); err != nil {
		t.Fatal(err)
	}

	// Watch the project, rolebindings, and configmaps
	nswatch, err := kubeClient.Namespaces().Watch(kapi.ListOptions{FieldSelector: fields.OneTermEqualSelector("metadata.name", ns)})
	if err != nil {
		t.Fatal(err)
	}
	policywatch, err := openshiftClient.PolicyBindings(ns).Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cmwatch, err := kubeClient.ConfigMaps(ns).Watch(kapi.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Create project request
	_, err = openshiftClient.ProjectRequests().Create(&projectapi.ProjectRequest{ObjectMeta: kapi.ObjectMeta{Name: ns}})
	if err == nil || err.Error() != `Internal error occurred: ConfigMap "" is invalid: metadata.name: Required value: name or generateName is required` {
		t.Fatalf("Expected internal error creating project, got %v", err)
	}

	pairCreationDeletion := func(w watch.Interface) (int, int, []watch.Event) {
		added := 0
		deleted := 0
		events := []watch.Event{}
		for {
			select {
			case e := <-w.ResultChan():
				events = append(events, e)
				switch e.Type {
				case watch.Added:
					added++
				case watch.Deleted:
					deleted++
				}
			case <-time.After(10 * time.Second):
				return added, deleted, events
			}

			if added == deleted && added > 0 {
				return added, deleted, events
			}
		}
	}

	if added, deleted, events := pairCreationDeletion(nswatch); added != deleted || added != 1 {
		for _, e := range events {
			t.Logf("%s %#v", e.Type, e.Object)
		}
		t.Errorf("expected 1 namespace to be added and deleted, got %d added / %d deleted", added, deleted)
	}
	if added, deleted, events := pairCreationDeletion(policywatch); added != deleted || added != 1 {
		for _, e := range events {
			t.Logf("%s %#v", e.Type, e.Object)
		}
		t.Errorf("expected 1 policybinding to be added and deleted, got %d added / %d deleted", added, deleted)
	}
	if added, deleted, events := pairCreationDeletion(cmwatch); added != deleted || added != 1 {
		for _, e := range events {
			t.Logf("%s %#v", e.Type, e.Object)
		}
		t.Errorf("expected 1 configmap to be added and deleted, got %d added / %d deleted", added, deleted)
	}

	// Verify project is deleted
	if nsObj, err := kubeClient.Namespaces().Get(ns); !kapierrors.IsNotFound(err) {
		t.Errorf("Expected namespace to be gone, got %#v, %#v", nsObj, err)
	}
}
