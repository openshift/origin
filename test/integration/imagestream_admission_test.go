package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	imagetest "github.com/openshift/origin/pkg/image/admission/testutil"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	quotautil "github.com/openshift/origin/pkg/quota/util"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

const limitRangeName = "limits"

var quotaExceededBackoff wait.Backoff

func init() {
	quotaExceededBackoff = retry.DefaultBackoff
	quotaExceededBackoff.Duration = time.Millisecond * 250
	quotaExceededBackoff.Steps = 3
}

func TestImageStreamTagsAdmission(t *testing.T) {
	kClient, client, fn := setupImageStreamAdmissionTest(t)
	defer fn()

	for i, name := range []string{imagetest.BaseImageWith1LayerDigest, imagetest.BaseImageWith2LayersDigest, imagetest.MiscImageDigest} {
		imageReference := fmt.Sprintf("openshift/test@%s", name)
		image := &imageapi.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			DockerImageReference: imageReference,
		}
		tag := fmt.Sprintf("tag%d", i+1)

		_, err := client.Image().ImageStreamMappings(testutil.Namespace()).Create(&imageapi.ImageStreamMapping{
			ObjectMeta: metav1.ObjectMeta{
				Name: "src",
			},
			Tag:   tag,
			Image: *image,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	limit := kapi.ResourceList{imageapi.ResourceImageStreamTags: resource.MustParse("0")}
	lrClient := kClient.Core().LimitRanges(testutil.Namespace())
	createLimitRangeOfType(t, lrClient, limitRangeName, imageapi.LimitTypeImageStream, limit)

	t.Logf("trying to create ImageStreamTag referencing isimage exceeding quota %v", limit)
	ist := &imageapi.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dest:tag1",
		},
		Tag: &imageapi.TagReference{
			Name: "1",
			From: &kapi.ObjectReference{
				Kind: "ImageStreamImage",
				Name: "src@" + imagetest.BaseImageWith1LayerDigest,
			},
		},
	}
	_, err := client.Image().ImageStreamTags(testutil.Namespace()).Update(ist)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !quotautil.IsErrorQuotaExceeded(err) {
		t.Errorf("expected quota exceeded error, got instead: %v", err)
	}

	limit = bumpLimit(t, lrClient, limitRangeName, imageapi.ResourceImageStreamTags, "1")

	t.Logf("trying to create ImageStreamTag referencing isimage below quota %v", limit)
	ist = &imageapi.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dest:tag1",
		},
		Tag: &imageapi.TagReference{
			Name: "1",
			From: &kapi.ObjectReference{
				Kind: "ImageStreamImage",
				Name: "src@" + imagetest.BaseImageWith1LayerDigest,
			},
		},
	}
	// we may hit cache with old limit, let's retry in such a case
	err = wait.ExponentialBackoff(quotaExceededBackoff, func() (bool, error) {
		_, err := client.Image().ImageStreamTags(testutil.Namespace()).Update(ist)
		if err != nil && !quotautil.IsErrorQuotaExceeded(err) {
			return false, err
		}
		return err == nil, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("trying to create ImageStreamTag exceeding quota %v", limit)
	ist = &imageapi.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dest:tag2",
		},
		Tag: &imageapi.TagReference{
			Name: "2",
			From: &kapi.ObjectReference{
				Kind: "ImageStreamImage",
				Name: "src@" + imagetest.BaseImageWith2LayersDigest,
			},
		},
	}
	_, err = client.Image().ImageStreamTags(testutil.Namespace()).Update(ist)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !quotautil.IsErrorQuotaExceeded(err) {
		t.Errorf("expected quota exceeded error, got instead: %v", err)
	}

	t.Log("trying to create ImageStreamTag referencing isimage already referenced")
	ist = &imageapi.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dest:tag1again",
		},
		Tag: &imageapi.TagReference{
			Name: "tag1again",
			From: &kapi.ObjectReference{
				Kind: "ImageStreamImage",
				Name: "src@" + imagetest.BaseImageWith1LayerDigest,
			},
		},
	}
	_, err = client.Image().ImageStreamTags(testutil.Namespace()).Update(ist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Log("trying to create ImageStreamTag in a new image stream")
	ist = &imageapi.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: "new:misc",
		},
		Tag: &imageapi.TagReference{
			Name: "misc",
			From: &kapi.ObjectReference{
				Kind: "ImageStreamImage",
				Name: "src@" + imagetest.MiscImageDigest,
			},
		},
	}
	_, err = client.Image().ImageStreamTags(testutil.Namespace()).Update(ist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	limit = bumpLimit(t, lrClient, limitRangeName, imageapi.ResourceImageStreamTags, "2")

	t.Logf("trying to create ImageStreamTag referencing istag below quota %v", limit)
	ist = &imageapi.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dest:tag2",
		},
		Tag: &imageapi.TagReference{
			Name: "2",
			From: &kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "src:tag2",
			},
		},
	}
	// we may hit cache with old limit, let's retry in such a case
	err = wait.ExponentialBackoff(quotaExceededBackoff, func() (bool, error) {
		_, err := client.Image().ImageStreamTags(testutil.Namespace()).Update(ist)
		if err != nil && !quotautil.IsErrorQuotaExceeded(err) {
			return false, err
		}
		return err == nil, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("trying to create ImageStreamTag referencing istag exceeding quota %v", limit)
	ist = &imageapi.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dest:tag3",
		},
		Tag: &imageapi.TagReference{
			Name: "3",
			From: &kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "src:tag3",
			},
		},
	}
	_, err = client.Image().ImageStreamTags(testutil.Namespace()).Update(ist)
	if err == nil {
		t.Fatal("creating image stream tag should have failed")
	}
	if !quotautil.IsErrorQuotaExceeded(err) {
		t.Fatalf("expected quota exceeded error, not: %v", err)
	}

	t.Log("trying to create ImageStreamTag referencing istag already referenced")
	ist = &imageapi.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dest:tag2again",
		},
		Tag: &imageapi.TagReference{
			Name: "tag2again",
			From: &kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "src:tag2",
			},
		},
	}
	_, err = client.Image().ImageStreamTags(testutil.Namespace()).Update(ist)
	if err != nil {
		t.Fatal(err)
	}
}

func TestImageStreamAdmitSpecUpdate(t *testing.T) {
	kClient, client, fn := setupImageStreamAdmissionTest(t)
	defer fn()

	for i, name := range []string{imagetest.BaseImageWith1LayerDigest, imagetest.BaseImageWith2LayersDigest} {
		imageReference := fmt.Sprintf("openshift/test@%s", name)
		image := &imageapi.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			DockerImageReference: imageReference,
		}
		tag := fmt.Sprintf("tag%d", i+1)

		_, err := client.Image().ImageStreamMappings(testutil.Namespace()).Create(&imageapi.ImageStreamMapping{
			ObjectMeta: metav1.ObjectMeta{
				Name: "src",
			},
			Tag:   tag,
			Image: *image,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	limit := kapi.ResourceList{
		imageapi.ResourceImageStreamTags:   resource.MustParse("0"),
		imageapi.ResourceImageStreamImages: resource.MustParse("0"),
	}
	lrClient := kClient.Core().LimitRanges(testutil.Namespace())
	createLimitRangeOfType(t, lrClient, limitRangeName, imageapi.LimitTypeImageStream, limit)

	t.Logf("trying to create a new image stream with a tag exceeding limit %v", limit)
	_, err := client.Image().ImageStreams(testutil.Namespace()).Create(
		newImageStreamWithSpecTags("is", map[string]kapi.ObjectReference{
			"tag1": {Kind: "ImageStreamTag", Name: "src:tag1"},
		}))

	if err == nil {
		t.Fatal("unexpected non-error")
	}
	if !quotautil.IsErrorQuotaExceeded(err) {
		t.Errorf("expected quota exceeded error, got instead: %#+v", err)
	}
	for _, res := range []kapi.ResourceName{imageapi.ResourceImageStreamTags, imageapi.ResourceImageStreamImages} {
		if !strings.Contains(err.Error(), string(res)) {
			t.Errorf("expected resource %q in error string: %v", res, err)
		}
	}

	limit = bumpLimit(t, lrClient, limitRangeName, imageapi.ResourceImageStreamTags, "1")
	limit = bumpLimit(t, lrClient, limitRangeName, imageapi.ResourceImageStreamImages, "1")

	t.Logf("trying to create a new image stream with a tag below limit %v", limit)
	// we may hit cache with old limit, let's retry in such a case
	err = wait.ExponentialBackoff(quotaExceededBackoff, func() (bool, error) {
		_, err = client.Image().ImageStreams(testutil.Namespace()).Create(
			newImageStreamWithSpecTags("is", map[string]kapi.ObjectReference{"tag1": {
				Kind: "ImageStreamTag",
				Name: "src:tag1",
			}}))
		if err != nil && !quotautil.IsErrorQuotaExceeded(err) {
			return false, err
		}
		return err == nil, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("adding new tag to image stream spec exceeding limit %v", limit)
	is, err := client.Image().ImageStreams(testutil.Namespace()).Get("is", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	is.Spec.Tags["tag2"] = imageapi.TagReference{
		Name: "tag2",
		From: &kapi.ObjectReference{
			Kind: "ImageStreamTag",
			Name: "src:tag2",
		},
	}
	_, err = client.Image().ImageStreams(testutil.Namespace()).Update(is)
	if err == nil {
		t.Fatalf("unexpected non-error")
	}
	if !quotautil.IsErrorQuotaExceeded(err) {
		t.Errorf("expected quota exceeded error, got instead: %v", err)
	}
	for _, res := range []kapi.ResourceName{imageapi.ResourceImageStreamTags, imageapi.ResourceImageStreamImages} {
		if !strings.Contains(err.Error(), string(res)) {
			t.Errorf("expected resource %q in error string: %v", res, err)
		}
	}

	t.Logf("re-tagging the image under different tag")
	is, err = client.Image().ImageStreams(testutil.Namespace()).Get("is", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	is.Spec.Tags["1again"] = imageapi.TagReference{
		Name: "1again",
		From: &kapi.ObjectReference{
			Kind: "ImageStreamTag",
			Name: "src:tag1",
		},
	}
	_, err = client.Image().ImageStreams(testutil.Namespace()).Update(is)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImageStreamAdmitStatusUpdate(t *testing.T) {
	kClient, client, fn := setupImageStreamAdmissionTest(t)
	defer fn()
	images := []*imageapi.Image{}

	for _, name := range []string{imagetest.BaseImageWith1LayerDigest, imagetest.BaseImageWith2LayersDigest} {
		imageReference := fmt.Sprintf("openshift/test@%s", name)
		image := &imageapi.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			DockerImageReference: imageReference,
		}
		images = append(images, image)

		_, err := client.Image().Images().Create(image)
		if err != nil {
			t.Fatal(err)
		}
	}

	limit := kapi.ResourceList{
		imageapi.ResourceImageStreamTags:   resource.MustParse("0"),
		imageapi.ResourceImageStreamImages: resource.MustParse("0"),
	}
	lrClient := kClient.Core().LimitRanges(testutil.Namespace())
	createLimitRangeOfType(t, lrClient, limitRangeName, imageapi.LimitTypeImageStream, limit)

	t.Logf("trying to create a new image stream with a tag exceeding limit %v", limit)
	_, err := client.Image().ImageStreams(testutil.Namespace()).Create(newImageStreamWithSpecTags("is", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("adding new tag to image stream status exceeding limit %v", limit)
	is, err := client.Image().ImageStreams(testutil.Namespace()).Get("is", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	is.Status.Tags["tag1"] = imageapi.TagEventList{
		Items: []imageapi.TagEvent{
			{
				DockerImageReference: images[0].DockerImageReference,
				Image:                images[0].Name,
			},
		},
	}
	_, err = client.Image().ImageStreams(testutil.Namespace()).UpdateStatus(is)
	if err == nil {
		t.Fatalf("unexpected non-error")
	}
	if !quotautil.IsErrorQuotaExceeded(err) {
		t.Errorf("expected quota exceeded error, got instead: %v", err)
	}
	if !strings.Contains(err.Error(), string(imageapi.ResourceImageStreamImages)) {
		t.Errorf("expected resource %q in error string: %v", imageapi.ResourceImageStreamImages, err)
	}

	limit = bumpLimit(t, lrClient, limitRangeName, imageapi.ResourceImageStreamImages, "1")

	t.Logf("adding new tag to image stream status below limit %v", limit)
	is, err = client.Image().ImageStreams(testutil.Namespace()).Get("is", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	is.Status.Tags["tag1"] = imageapi.TagEventList{
		Items: []imageapi.TagEvent{
			{
				DockerImageReference: images[0].DockerImageReference,
				Image:                images[0].Name,
			},
		},
	}
	_, err = client.Image().ImageStreams(testutil.Namespace()).UpdateStatus(is)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("adding new tag to image stream status exceeding limit %v", limit)
	is, err = client.Image().ImageStreams(testutil.Namespace()).Get("is", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	is.Status.Tags["tag2"] = imageapi.TagEventList{
		Items: []imageapi.TagEvent{
			{
				DockerImageReference: images[1].DockerImageReference,
				Image:                images[1].Name,
			},
		},
	}
	_, err = client.Image().ImageStreams(testutil.Namespace()).UpdateStatus(is)
	if err == nil {
		t.Fatalf("unexpected non-error")
	}
	if !quotautil.IsErrorQuotaExceeded(err) {
		t.Errorf("expected quota exceeded error, got instead: %v", err)
	}
	if !strings.Contains(err.Error(), string(imageapi.ResourceImageStreamImages)) {
		t.Errorf("expected resource %q in error string: %v", imageapi.ResourceImageStreamImages, err)
	}

	t.Logf("re-tagging the image under different tag")
	is, err = client.Image().ImageStreams(testutil.Namespace()).Get("is", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	is.Status.Tags["1again"] = imageapi.TagEventList{
		Items: []imageapi.TagEvent{
			{
				DockerImageReference: images[0].DockerImageReference,
				Image:                images[0].Name,
			},
		},
	}
	_, err = client.Image().ImageStreams(testutil.Namespace()).UpdateStatus(is)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func setupImageStreamAdmissionTest(t *testing.T) (kclientset.Interface, imageclient.Interface, func()) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	clusterAdminImageClient := imageclient.NewForConfigOrDie(clusterAdminClientConfig)
	err = testutil.CreateNamespace(clusterAdminKubeConfig, testutil.Namespace())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	for {
		_, err = clusterAdminImageClient.Image().ImageStreams(testutil.Namespace()).Create(newImageStreamWithSpecTags("src", nil))
		t.Logf("initing: %v", err)
		if err != nil {
			if errForbiddenWithRetry(err) {
				t.Logf("waiting for limit ranger to catch up: %v", err)
				continue
			}
			t.Fatalf("err: %#v", err)
		}
		break
	}
	return kClient, clusterAdminImageClient, func() {
		testserver.CleanupMasterEtcd(t, masterConfig)
	}
}

// errForbiddenWithRetry returns true if this is a status error and has requested a retry
func errForbiddenWithRetry(err error) bool {
	if err == nil || !kapierrors.IsForbidden(err) {
		return false
	}
	status, ok := err.(kapierrors.APIStatus)
	if !ok {
		return false
	}
	return status.Status().Details != nil && status.Status().Details.RetryAfterSeconds > 0
}

// createResourceQuota creates a resource quota with given hard limits in a current namespace and waits until
// a first usage refresh
func createResourceQuota(t *testing.T, rqClient kcoreclient.ResourceQuotaInterface, quotaName string, hard kapi.ResourceList) *kapi.ResourceQuota {
	rq := &kapi.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name: quotaName,
		},
		Spec: kapi.ResourceQuotaSpec{
			Hard: hard,
		},
	}

	t.Logf("creating resource quota %q with a limit %v", quotaName, hard)
	rq, err := rqClient.Create(rq)
	if err != nil {
		t.Fatal(err)
	}
	err = testutil.WaitForResourceQuotaLimitSync(rqClient, quotaName, hard, time.Second*30)
	if err != nil {
		t.Fatal(err)
	}
	return rq
}

// bumpQuota modifies hard spec of quota object with the given value. It returns modified hard spec.
func bumpQuota(t *testing.T, rqs kcoreclient.ResourceQuotaInterface, quotaName string, resourceName kapi.ResourceName, value int64) kapi.ResourceList {
	t.Logf("bump the quota %s to %s=%d", quotaName, resourceName, value)
	rq, err := rqs.Get(quotaName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rq.Spec.Hard[resourceName] = *resource.NewQuantity(value, resource.DecimalSI)
	_, err = rqs.Update(rq)
	if err != nil {
		t.Fatal(err)
	}
	err = testutil.WaitForResourceQuotaLimitSync(
		rqs,
		quotaName,
		rq.Spec.Hard,
		time.Second*10)
	if err != nil {
		t.Fatal(err)
	}
	return rq.Spec.Hard
}

// createLimitRangeOfType creates a new limit range object with given max limits set for given limit type. The
// object will be created in current namespace.
func createLimitRangeOfType(t *testing.T, lrClient kcoreclient.LimitRangeInterface, limitRangeName string, limitType kapi.LimitType, maxLimits kapi.ResourceList) *kapi.LimitRange {
	lr := &kapi.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name: limitRangeName,
		},
		Spec: kapi.LimitRangeSpec{
			Limits: []kapi.LimitRangeItem{
				{
					Type: limitType,
					Max:  maxLimits,
				},
			},
		},
	}

	t.Logf("creating limit range object %q with %s limited to: %v", limitRangeName, limitType, maxLimits)
	lr, err := lrClient.Create(lr)
	if err != nil {
		t.Fatal(err)
	}
	return lr
}

func bumpLimit(t *testing.T, lrClient kcoreclient.LimitRangeInterface, limitRangeName string, resourceName kapi.ResourceName, limit string) kapi.ResourceList {
	t.Logf("bump a limit on resource %q to %s", resourceName, limit)
	lr, err := lrClient.Get(limitRangeName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	res := kapi.ResourceList{}

	change := false
	for i := range lr.Spec.Limits {
		item := &lr.Spec.Limits[i]
		if old, exists := item.Max[resourceName]; exists {
			for k, v := range item.Max {
				res[k] = v
			}
			parsed := resource.MustParse(limit)
			if old.Cmp(parsed) != 0 {
				item.Max[resourceName] = parsed
				change = true
			}
		}
	}

	if !change {
		return res
	}
	_, err = lrClient.Update(lr)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func newImageStreamWithSpecTags(name string, tags map[string]kapi.ObjectReference) *imageapi.ImageStream {
	specTags := make(map[string]imageapi.TagReference)
	for tag := range tags {
		ref := tags[tag]
		specTags[tag] = imageapi.TagReference{Name: tag, From: &ref}
	}
	return &imageapi.ImageStream{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       imageapi.ImageStreamSpec{Tags: specTags},
	}

}
