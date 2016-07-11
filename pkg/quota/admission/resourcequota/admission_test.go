package resourcequota

import (
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	kfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/quota"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

// TestOriginQuotaAdmissionIsErrorQuotaExceeded verifies that if a resource exceeds allowed usage, the
// admission will return error we can recognize.
func TestOriginQuotaAdmissionIsErrorQuotaExceeded(t *testing.T) {
	resourceQuota := &kapi.ResourceQuota{
		ObjectMeta: kapi.ObjectMeta{Name: "quota", Namespace: "test", ResourceVersion: "124"},
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
	osClient := testclient.NewSimpleFake(&imageapi.ImageStream{})
	plugin := NewOriginResourceQuota(kubeClient).(*originQuotaAdmission)
	plugin.SetOriginQuotaRegistry(quota.NewOriginQuotaRegistry(osClient))
	if err := plugin.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newIS := &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "test",
			Name:      "is",
		},
	}

	err := plugin.Admit(admission.NewAttributesRecord(newIS, nil, imageapi.Kind("ImageStream").WithVersion("version"), newIS.Namespace, newIS.Name, kapi.Resource("imageStreams").WithVersion("version"), "", admission.Create, nil))
	if err == nil {
		t.Fatalf("Expected an error exceeding quota")
	}
	if !quotautil.IsErrorQuotaExceeded(err) {
		t.Fatalf("Expected error %q to be matched by IsErrorQuotaExceeded()", err.Error())
	}
}
