package images

import (
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/util/retry"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	imagev1 "github.com/openshift/api/image/v1"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned"
	"github.com/openshift/library-go/pkg/quota/quotautil"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Image][triggers][Serial] ImageStream admission", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("imagestream-admission", exutil.KubeConfigPath())

	g.It("TestImageStreamTagsAdmission", func() {
		TestImageStreamTagsAdmission(g.GinkgoT(), oc)
	})
	g.It("TestImageStreamAdmitSpecUpdate", func() {
		TestImageStreamAdmitSpecUpdate(g.GinkgoT(), oc)
	})
	g.It("TestImageStreamAdmitStatusUpdate", func() {
		TestImageStreamAdmitStatusUpdate(g.GinkgoT(), oc)
	})
})

const limitRangeName = "limits"

var quotaExceededBackoff wait.Backoff

func init() {
	quotaExceededBackoff = retry.DefaultBackoff
	quotaExceededBackoff.Duration = time.Millisecond * 250
	quotaExceededBackoff.Steps = 3
}

func TestImageStreamTagsAdmission(t g.GinkgoTInterface, oc *exutil.CLI) {
	kClient, client, fn := setupImageStreamAdmissionTest(t, oc)
	defer fn()

	for i, name := range []string{BaseImageWith1LayerDigest, BaseImageWith2LayersDigest, MiscImageDigest} {
		imageReference := fmt.Sprintf("openshift/test@%s", name)
		image := &imagev1.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			DockerImageReference: imageReference,
		}
		tag := fmt.Sprintf("tag%d", i+1)

		oc.AddExplicitResourceToDelete(imagev1.GroupVersion.WithResource("images"), "", name)
		_, err := client.ImageV1().ImageStreamMappings(oc.Namespace()).Create(&imagev1.ImageStreamMapping{
			ObjectMeta: metav1.ObjectMeta{
				Name: "src",
			},
			Tag:   tag,
			Image: *image,
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	limit := corev1.ResourceList{imagev1.ResourceImageStreamTags: resource.MustParse("0")}
	lrClient := kClient.CoreV1().LimitRanges(oc.Namespace())
	createLimitRangeOfType(t, oc, lrClient, limitRangeName, imagev1.LimitTypeImageStream, limit)

	t.Logf("trying to create ImageStreamTag referencing isimage exceeding quota %v", limit)
	ist := &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dest:tag1",
		},
		Tag: &imagev1.TagReference{
			Name: "1",
			From: &corev1.ObjectReference{
				Kind: "ImageStreamImage",
				Name: "src@" + BaseImageWith1LayerDigest,
			},
		},
	}
	_, err := client.ImageV1().ImageStreamTags(oc.Namespace()).Update(ist)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !quotautil.IsErrorQuotaExceeded(err) {
		t.Errorf("expected quota exceeded error, got instead: %v", err)
	}

	limit = bumpLimit(t, oc, lrClient, limitRangeName, imagev1.ResourceImageStreamTags, "1")

	t.Logf("trying to create ImageStreamTag referencing isimage below quota %v", limit)
	ist = &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dest:tag1",
		},
		Tag: &imagev1.TagReference{
			Name: "1",
			From: &corev1.ObjectReference{
				Kind: "ImageStreamImage",
				Name: "src@" + BaseImageWith1LayerDigest,
			},
		},
	}
	// we may hit cache with old limit, let's retry in such a case
	err = wait.ExponentialBackoff(quotaExceededBackoff, func() (bool, error) {
		_, err := client.ImageV1().ImageStreamTags(oc.Namespace()).Update(ist)
		if err != nil && !quotautil.IsErrorQuotaExceeded(err) {
			return false, err
		}
		return err == nil, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("trying to create ImageStreamTag exceeding quota %v", limit)
	ist = &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dest:tag2",
		},
		Tag: &imagev1.TagReference{
			Name: "2",
			From: &corev1.ObjectReference{
				Kind: "ImageStreamImage",
				Name: "src@" + BaseImageWith2LayersDigest,
			},
		},
	}
	_, err = client.ImageV1().ImageStreamTags(oc.Namespace()).Update(ist)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !quotautil.IsErrorQuotaExceeded(err) {
		t.Errorf("expected quota exceeded error, got instead: %v", err)
	}

	t.Log("trying to create ImageStreamTag referencing isimage already referenced")
	ist = &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dest:tag1again",
		},
		Tag: &imagev1.TagReference{
			Name: "tag1again",
			From: &corev1.ObjectReference{
				Kind: "ImageStreamImage",
				Name: "src@" + BaseImageWith1LayerDigest,
			},
		},
	}
	_, err = client.ImageV1().ImageStreamTags(oc.Namespace()).Update(ist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Log("trying to create ImageStreamTag in a new image stream")
	ist = &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: "new:misc",
		},
		Tag: &imagev1.TagReference{
			Name: "misc",
			From: &corev1.ObjectReference{
				Kind: "ImageStreamImage",
				Name: "src@" + MiscImageDigest,
			},
		},
	}
	_, err = client.ImageV1().ImageStreamTags(oc.Namespace()).Update(ist)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	limit = bumpLimit(t, oc, lrClient, limitRangeName, imagev1.ResourceImageStreamTags, "2")

	t.Logf("trying to create ImageStreamTag referencing istag below quota %v", limit)
	ist = &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dest:tag2",
		},
		Tag: &imagev1.TagReference{
			Name: "2",
			From: &corev1.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "src:tag2",
			},
		},
	}
	// we may hit cache with old limit, let's retry in such a case
	err = wait.ExponentialBackoff(quotaExceededBackoff, func() (bool, error) {
		_, err := client.ImageV1().ImageStreamTags(oc.Namespace()).Update(ist)
		if err != nil && !quotautil.IsErrorQuotaExceeded(err) {
			return false, err
		}
		return err == nil, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("trying to create ImageStreamTag referencing istag exceeding quota %v", limit)
	ist = &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dest:tag3",
		},
		Tag: &imagev1.TagReference{
			Name: "3",
			From: &corev1.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "src:tag3",
			},
		},
	}
	_, err = client.ImageV1().ImageStreamTags(oc.Namespace()).Update(ist)
	if err == nil {
		t.Fatal("creating image stream tag should have failed")
	}
	if !quotautil.IsErrorQuotaExceeded(err) {
		t.Fatalf("expected quota exceeded error, not: %v", err)
	}

	t.Log("trying to create ImageStreamTag referencing istag already referenced")
	ist = &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dest:tag2again",
		},
		Tag: &imagev1.TagReference{
			Name: "tag2again",
			From: &corev1.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "src:tag2",
			},
		},
	}
	_, err = client.ImageV1().ImageStreamTags(oc.Namespace()).Update(ist)
	if err != nil {
		t.Fatal(err)
	}
}

func TestImageStreamAdmitSpecUpdate(t g.GinkgoTInterface, oc *exutil.CLI) {
	kClient, client, fn := setupImageStreamAdmissionTest(t, oc)
	defer fn()

	for i, name := range []string{BaseImageWith1LayerDigest, BaseImageWith2LayersDigest} {
		imageReference := fmt.Sprintf("openshift/test@%s", name)
		image := &imagev1.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			DockerImageReference: imageReference,
		}
		tag := fmt.Sprintf("tag%d", i+1)

		oc.AddExplicitResourceToDelete(imagev1.GroupVersion.WithResource("images"), "", name)
		_, err := client.ImageV1().ImageStreamMappings(oc.Namespace()).Create(&imagev1.ImageStreamMapping{
			ObjectMeta: metav1.ObjectMeta{
				Name: "src",
			},
			Tag:   tag,
			Image: *image,
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	limit := corev1.ResourceList{
		imagev1.ResourceImageStreamTags:   resource.MustParse("0"),
		imagev1.ResourceImageStreamImages: resource.MustParse("0"),
	}
	lrClient := kClient.CoreV1().LimitRanges(oc.Namespace())
	createLimitRangeOfType(t, oc, lrClient, limitRangeName, imagev1.LimitTypeImageStream, limit)

	t.Logf("trying to create a new image stream with a tag exceeding limit %v", limit)
	_, err := client.ImageV1().ImageStreams(oc.Namespace()).Create(
		newImageStreamWithSpecTags("is", map[string]corev1.ObjectReference{
			"tag1": {Kind: "ImageStreamTag", Name: "src:tag1"},
		}))

	if err == nil {
		t.Fatal("unexpected non-error")
	}
	if !quotautil.IsErrorQuotaExceeded(err) {
		t.Errorf("expected quota exceeded error, got instead: %#+v", err)
	}
	for _, res := range []corev1.ResourceName{imagev1.ResourceImageStreamTags, imagev1.ResourceImageStreamImages} {
		if !strings.Contains(err.Error(), string(res)) {
			t.Errorf("expected resource %q in error string: %v", res, err)
		}
	}

	limit = bumpLimit(t, oc, lrClient, limitRangeName, imagev1.ResourceImageStreamTags, "1")
	limit = bumpLimit(t, oc, lrClient, limitRangeName, imagev1.ResourceImageStreamImages, "1")

	t.Logf("trying to create a new image stream with a tag below limit %v", limit)
	// we may hit cache with old limit, let's retry in such a case
	err = wait.ExponentialBackoff(quotaExceededBackoff, func() (bool, error) {
		_, err = client.ImageV1().ImageStreams(oc.Namespace()).Create(
			newImageStreamWithSpecTags("is", map[string]corev1.ObjectReference{"tag1": {
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
	is, err := client.ImageV1().ImageStreams(oc.Namespace()).Get("is", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	upsertSpecTag(&is.Spec.Tags, imagev1.TagReference{
		Name: "tag2",
		From: &corev1.ObjectReference{
			Kind: "ImageStreamTag",
			Name: "src:tag2",
		},
	})
	_, err = client.ImageV1().ImageStreams(oc.Namespace()).Update(is)
	if err == nil {
		t.Fatalf("unexpected non-error")
	}
	if !quotautil.IsErrorQuotaExceeded(err) {
		t.Errorf("expected quota exceeded error, got instead: %v", err)
	}
	for _, res := range []corev1.ResourceName{imagev1.ResourceImageStreamTags, imagev1.ResourceImageStreamImages} {
		if !strings.Contains(err.Error(), string(res)) {
			t.Errorf("expected resource %q in error string: %v", res, err)
		}
	}

	t.Logf("re-tagging the image under different tag")
	is, err = client.ImageV1().ImageStreams(oc.Namespace()).Get("is", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	upsertSpecTag(&is.Spec.Tags, imagev1.TagReference{
		Name: "1again",
		From: &corev1.ObjectReference{
			Kind: "ImageStreamTag",
			Name: "src:tag1",
		},
	})
	_, err = client.ImageV1().ImageStreams(oc.Namespace()).Update(is)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func upsertSpecTag(tags *[]imagev1.TagReference, tagReference imagev1.TagReference) {
	for i := range *tags {
		curr := (*tags)[i]
		if curr.Name == tagReference.Name {
			(*tags)[i] = tagReference
			return
		}
	}
	*tags = append(*tags, tagReference)
}

func upsertStatusTag(tags *[]imagev1.NamedTagEventList, tagEventList imagev1.NamedTagEventList) {
	for i := range *tags {
		curr := (*tags)[i]
		if curr.Tag == tagEventList.Tag {
			(*tags)[i] = tagEventList
			return
		}
	}
	*tags = append(*tags, tagEventList)
}

func TestImageStreamAdmitStatusUpdate(t g.GinkgoTInterface, oc *exutil.CLI) {
	kClient, client, fn := setupImageStreamAdmissionTest(t, oc)
	defer fn()
	images := []*imagev1.Image{}

	for _, name := range []string{BaseImageWith1LayerDigest, BaseImageWith2LayersDigest} {
		imageReference := fmt.Sprintf("openshift/test@%s", name)
		image := &imagev1.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			DockerImageReference: imageReference,
		}
		images = append(images, image)

		oc.AddExplicitResourceToDelete(imagev1.GroupVersion.WithResource("images"), "", name)
		_, err := client.ImageV1().Images().Create(image)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	}

	limit := corev1.ResourceList{
		imagev1.ResourceImageStreamTags:   resource.MustParse("0"),
		imagev1.ResourceImageStreamImages: resource.MustParse("0"),
	}
	lrClient := kClient.CoreV1().LimitRanges(oc.Namespace())
	createLimitRangeOfType(t, oc, lrClient, limitRangeName, imagev1.LimitTypeImageStream, limit)

	t.Logf("trying to create a new image stream with a tag exceeding limit %v", limit)
	_, err := client.ImageV1().ImageStreams(oc.Namespace()).Create(newImageStreamWithSpecTags("is", nil))
	o.Expect(err).NotTo(o.HaveOccurred())

	t.Logf("adding new tag to image stream status exceeding limit %v", limit)
	is, err := client.ImageV1().ImageStreams(oc.Namespace()).Get("is", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	upsertStatusTag(&is.Status.Tags, imagev1.NamedTagEventList{
		Tag: "tag1",
		Items: []imagev1.TagEvent{
			{
				DockerImageReference: images[0].DockerImageReference,
				Image:                images[0].Name,
			},
		},
	})
	_, err = client.ImageV1().ImageStreams(oc.Namespace()).UpdateStatus(is)
	if err == nil {
		t.Fatalf("unexpected non-error")
	}
	if !quotautil.IsErrorQuotaExceeded(err) {
		t.Errorf("expected quota exceeded error, got instead: %v", err)
	}
	if !strings.Contains(err.Error(), string(imagev1.ResourceImageStreamImages)) {
		t.Errorf("expected resource %q in error string: %v", imagev1.ResourceImageStreamImages, err)
	}

	limit = bumpLimit(t, oc, lrClient, limitRangeName, imagev1.ResourceImageStreamImages, "1")

	t.Logf("adding new tag to image stream status below limit %v", limit)
	is, err = client.ImageV1().ImageStreams(oc.Namespace()).Get("is", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	upsertStatusTag(&is.Status.Tags, imagev1.NamedTagEventList{
		Tag: "tag1",
		Items: []imagev1.TagEvent{
			{
				DockerImageReference: images[0].DockerImageReference,
				Image:                images[0].Name,
			},
		},
	})
	_, err = client.ImageV1().ImageStreams(oc.Namespace()).UpdateStatus(is)
	o.Expect(err).NotTo(o.HaveOccurred())

	t.Logf("adding new tag to image stream status exceeding limit %v", limit)
	is, err = client.ImageV1().ImageStreams(oc.Namespace()).Get("is", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	upsertStatusTag(&is.Status.Tags, imagev1.NamedTagEventList{
		Tag: "tag2",
		Items: []imagev1.TagEvent{
			{
				DockerImageReference: images[1].DockerImageReference,
				Image:                images[1].Name,
			},
		},
	})
	_, err = client.ImageV1().ImageStreams(oc.Namespace()).UpdateStatus(is)
	if err == nil {
		t.Fatalf("unexpected non-error")
	}
	if !quotautil.IsErrorQuotaExceeded(err) {
		t.Errorf("expected quota exceeded error, got instead: %v", err)
	}
	if !strings.Contains(err.Error(), string(imagev1.ResourceImageStreamImages)) {
		t.Errorf("expected resource %q in error string: %v", imagev1.ResourceImageStreamImages, err)
	}

	t.Logf("re-tagging the image under different tag")
	is, err = client.ImageV1().ImageStreams(oc.Namespace()).Get("is", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	upsertStatusTag(&is.Status.Tags, imagev1.NamedTagEventList{
		Tag: "1again",
		Items: []imagev1.TagEvent{
			{
				DockerImageReference: images[0].DockerImageReference,
				Image:                images[0].Name,
			},
		},
	})
	_, err = client.ImageV1().ImageStreams(oc.Namespace()).UpdateStatus(is)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func setupImageStreamAdmissionTest(t g.GinkgoTInterface, oc *exutil.CLI) (kubernetes.Interface, imagev1client.Interface, func()) {
	for {
		_, err := oc.AdminImageClient().ImageV1().ImageStreams(oc.Namespace()).Create(newImageStreamWithSpecTags("src", nil))
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
	return oc.AdminKubeClient(), oc.AdminImageClient(), func() {}
}

// errForbiddenWithRetry returns true if this is a status error and has requested a retry
func errForbiddenWithRetry(err error) bool {
	if err == nil || !apierrors.IsForbidden(err) {
		return false
	}
	status, ok := err.(apierrors.APIStatus)
	if !ok {
		return false
	}
	return status.Status().Details != nil && status.Status().Details.RetryAfterSeconds > 0
}

// createLimitRangeOfType creates a new limit range object with given max limits set for given limit type. The
// object will be created in current namespace.
func createLimitRangeOfType(t g.GinkgoTInterface, oc *exutil.CLI, lrClient corev1client.LimitRangeInterface, limitRangeName string, limitType corev1.LimitType, maxLimits corev1.ResourceList) *corev1.LimitRange {
	lr := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name: limitRangeName,
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
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

func bumpLimit(t g.GinkgoTInterface, oc *exutil.CLI, lrClient corev1client.LimitRangeInterface, limitRangeName string, resourceName corev1.ResourceName, limit string) corev1.ResourceList {
	t.Logf("bump a limit on resource %q to %s", resourceName, limit)
	lr, err := lrClient.Get(limitRangeName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	res := corev1.ResourceList{}

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

func newImageStreamWithSpecTags(name string, tags map[string]corev1.ObjectReference) *imagev1.ImageStream {
	specTags := []imagev1.TagReference{}
	for tag := range tags {
		ref := tags[tag]
		specTags = append(specTags, imagev1.TagReference{Name: tag, From: &ref})
	}
	return &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       imagev1.ImageStreamSpec{Tags: specTags},
	}

}
