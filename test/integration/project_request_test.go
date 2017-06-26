package integration

import (
	"testing"
	"time"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/api"
	kapiv1 "k8s.io/kubernetes/pkg/api/v1"

	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	"github.com/openshift/origin/pkg/project/registry/projectrequest/delegated"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
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
	kubeClientset, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
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
		&kapi.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "configmapname"}},
		// Append a custom object that will fail validation
		&kapi.ConfigMap{},
		// Append another object that should never be created, since we short circuit
		&kapi.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "configmapname2"}},
	}
	if err := templateapi.AddObjectsToTemplate(template, additionalObjects, kapiv1.SchemeGroupVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := openshiftClient.Templates(templateNamespace).Create(template); err != nil {
		t.Fatal(err)
	}

	// Watch the project, rolebindings, and configmaps
	nswatch, err := kubeClientset.Core().Namespaces().Watch(metav1.ListOptions{FieldSelector: fields.OneTermEqualSelector("metadata.name", ns).String()})
	if err != nil {
		t.Fatal(err)
	}
	policywatch, err := openshiftClient.PolicyBindings(ns).Watch(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cmwatch, err := kubeClientset.Core().ConfigMaps(ns).Watch(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Create project request
	_, err = openshiftClient.ProjectRequests().Create(&projectapi.ProjectRequest{ObjectMeta: metav1.ObjectMeta{Name: ns}})
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
	if nsObj, err := kubeClientset.Core().Namespaces().Get(ns, metav1.GetOptions{}); !kapierrors.IsNotFound(err) {
		t.Errorf("Expected namespace to be gone, got %#v, %#v", nsObj, err)
	}
}
