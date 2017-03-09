package resourcequota

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/controller/informers"

	"github.com/openshift/origin/pkg/client/testclient"
	"github.com/openshift/origin/pkg/controller/shared"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/quota"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

// TestOriginQuotaAdmissionIsErrorQuotaExceeded verifies that if a resource exceeds allowed usage, the
// admission will return error we can recognize.
func TestOriginQuotaAdmissionIsErrorQuotaExceeded(t *testing.T) {
	resourceQuota := &kapi.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "quota", Namespace: "test", ResourceVersion: "124"},
		Status: kapi.ResourceQuotaStatus{
			Hard: kapi.ResourceList{
				imageapi.ResourceImageStreams: resource.MustParse("0"),
			},
			Used: kapi.ResourceList{
				imageapi.ResourceImageStreams: resource.MustParse("0"),
			},
		},
	}
	kubeClient := kfake.NewSimpleClientset(resourceQuota)
	osClient := testclient.NewSimpleFake()
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 10*time.Minute)
	informerFactory := shared.NewInformerFactory(kubeInformerFactory, kubeClient, osClient, shared.DefaultListerWatcherOverrides{}, 10*time.Minute)
	plugin := NewOriginResourceQuota(kubeClient).(*originQuotaAdmission)
	plugin.SetOriginQuotaRegistry(quota.NewOriginQuotaRegistry(informerFactory.ImageStreams(), osClient))
	if err := plugin.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newIS := &imageapi.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "is",
		},
	}

	err := plugin.Admit(admission.NewAttributesRecord(newIS, nil, imageapi.LegacyKind("ImageStream").WithVersion("version"), newIS.Namespace, newIS.Name, kapi.Resource("imageStreams").WithVersion("version"), "", admission.Create, nil))
	if err == nil {
		t.Fatalf("Expected an error exceeding quota")
	}
	if !quotautil.IsErrorQuotaExceeded(err) {
		t.Fatalf("Expected error %q to be matched by IsErrorQuotaExceeded()", err.Error())
	}
}
