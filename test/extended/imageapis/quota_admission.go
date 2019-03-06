package imageapis

import (
	"fmt"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	imageapiv1 "github.com/openshift/api/image/v1"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
)

const (
	quotaName   = "isquota"
	waitTimeout = time.Second * 600
)

var _ = g.Describe("[Feature:ImageQuota][registry] Image resource quota", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("resourcequota-admission", exutil.KubeConfigPath())

	g.It(fmt.Sprintf("should deny a push of built image exceeding %s quota", imageapi.ResourceImageStreams), func() {
		g.Skip("TODO: determine why this test is not skipped/fails on 4.0 clusters")
		quota := kapi.ResourceList{
			imageapi.ResourceImageStreams: resource.MustParse("0"),
		}
		_, err := createResourceQuota(oc, quota)
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err := waitForResourceQuotaSync(oc, quotaName, quota)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, quota)).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image exceeding quota %v", quota))
		err = createImageStreamMapping(oc, oc.Namespace(), "first", "refused")
		assertQuotaExceeded(err)

		quota, err = bumpQuota(oc, imageapi.ResourceImageStreams, 1)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image below quota %v", quota))
		err = createImageStreamMapping(oc, oc.Namespace(), "first", "tag1")
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err = waitForResourceQuotaSync(oc, quotaName, quota)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, quota)).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image to existing image stream %v", quota))
		err = createImageStreamMapping(oc, oc.Namespace(), "first", "tag2")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image exceeding quota %v", quota))
		err = createImageStreamMapping(oc, oc.Namespace(), "second", "refused")
		assertQuotaExceeded(err)

		quota, err = bumpQuota(oc, imageapi.ResourceImageStreams, 2)
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err = waitForResourceQuotaSync(oc, quotaName, used)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image below quota %v", quota))
		err = createImageStreamMapping(oc, oc.Namespace(), "second", "tag1")
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err = waitForResourceQuotaSync(oc, quotaName, quota)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, quota)).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image exceeding quota %v", quota))
		err = createImageStreamMapping(oc, oc.Namespace(), "third", "refused")
		assertQuotaExceeded(err)

		g.By("deleting first image stream")
		err = oc.ImageClient().Image().ImageStreams(oc.Namespace()).Delete("first", nil)
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err = exutil.WaitForResourceQuotaSync(
			oc.InternalKubeClient().Core().ResourceQuotas(oc.Namespace()),
			quotaName,
			kapi.ResourceList{imageapi.ResourceImageStreams: resource.MustParse("1")},
			true,
			waitTimeout,
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, kapi.ResourceList{imageapi.ResourceImageStreams: resource.MustParse("1")})).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image below quota %v", quota))
		err = createImageStreamMapping(oc, oc.Namespace(), "third", "tag")
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err = waitForResourceQuotaSync(oc, quotaName, quota)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, quota)).NotTo(o.HaveOccurred())
	})
})

// createResourceQuota creates a resource quota with given hard limits in a current namespace and waits until
// a first usage refresh
func createResourceQuota(oc *exutil.CLI, hard kapi.ResourceList) (*kapi.ResourceQuota, error) {
	rq := &kapi.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name: quotaName,
		},
		Spec: kapi.ResourceQuotaSpec{
			Hard: hard,
		},
	}

	g.By(fmt.Sprintf("creating resource quota with a limit %v", hard))
	rq, err := oc.InternalAdminKubeClient().Core().ResourceQuotas(oc.Namespace()).Create(rq)
	if err != nil {
		return nil, err
	}
	err = waitForLimitSync(oc, hard)
	return rq, err
}

// assertQuotasEqual compares two quota sets and returns an error with proper description when they don't match
func assertQuotasEqual(a, b kapi.ResourceList) error {
	errs := []error{}
	if len(a) != len(b) {
		errs = append(errs, fmt.Errorf("number of items does not match (%d != %d)", len(a), len(b)))
	}

	for k, av := range a {
		if bv, exists := b[k]; exists {
			if av.Cmp(bv) != 0 {
				errs = append(errs, fmt.Errorf("a[%s] != b[%s] (%s != %s)", k, k, av.String(), bv.String()))
			}
		} else {
			errs = append(errs, fmt.Errorf("resource %q not present in b", k))
		}
	}

	for k := range b {
		if _, exists := a[k]; !exists {
			errs = append(errs, fmt.Errorf("resource %q not present in a", k))
		}
	}

	return kutilerrors.NewAggregate(errs)
}

// bumpQuota modifies hard spec of quota object with the given value. It returns modified hard spec.
func bumpQuota(oc *exutil.CLI, resourceName kapi.ResourceName, value int64) (kapi.ResourceList, error) {
	g.By(fmt.Sprintf("bump the quota to %s=%d", resourceName, value))
	rq, err := oc.InternalAdminKubeClient().Core().ResourceQuotas(oc.Namespace()).Get(quotaName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	rq.Spec.Hard[resourceName] = *resource.NewQuantity(value, resource.DecimalSI)
	_, err = oc.InternalAdminKubeClient().Core().ResourceQuotas(oc.Namespace()).Update(rq)
	if err != nil {
		return nil, err
	}
	err = waitForLimitSync(oc, rq.Spec.Hard)
	if err != nil {
		return nil, err
	}
	return rq.Spec.Hard, nil
}

// waitForResourceQuotaSync waits until a usage of a quota reaches given limit with a short timeout
func waitForResourceQuotaSync(oc *exutil.CLI, name string, expectedResources kapi.ResourceList) (kapi.ResourceList, error) {
	g.By(fmt.Sprintf("waiting for resource quota %s to get updated", name))
	used, err := exutil.WaitForResourceQuotaSync(
		oc.InternalKubeClient().Core().ResourceQuotas(oc.Namespace()),
		quotaName,
		expectedResources,
		false,
		waitTimeout,
	)
	if err != nil {
		return nil, err
	}
	return used, nil
}

// waitForLimitSync waits until a usage of a quota reaches given limit with a short timeout
func waitForLimitSync(oc *exutil.CLI, hardLimit kapi.ResourceList) error {
	g.By(fmt.Sprintf("waiting for resource quota %s to get updated", quotaName))
	return testutil.WaitForResourceQuotaLimitSync(
		oc.InternalKubeClient().Core().ResourceQuotas(oc.Namespace()),
		quotaName,
		hardLimit,
		waitTimeout)
}

func createImageStreamMapping(oc *exutil.CLI, namespace, name, tag string) error {
	e2e.Logf("Creating image stream mapping for %s/%s:%s...", namespace, name, tag)
	_, err := oc.AdminImageClient().Image().ImageStreams(namespace).Get(name, metav1.GetOptions{})
	if kerrors.IsNotFound(err) {
		_, err = oc.AdminImageClient().Image().ImageStreams(namespace).Create(&imageapiv1.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		})
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	_, err = oc.AdminImageClient().Image().ImageStreamMappings(namespace).Create(&imageapiv1.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Image: imageapiv1.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
		},
		Tag: tag,
	})
	return err
}

func assertQuotaExceeded(err error) {
	o.Expect(kerrors.ReasonForError(err)).To(o.Equal(metav1.StatusReasonForbidden))
	o.Expect(err.Error()).To(o.ContainSubstring("exceeded quota"))
}
