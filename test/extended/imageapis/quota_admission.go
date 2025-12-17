package imageapis

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	quota "k8s.io/apiserver/pkg/quota/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	imagev1 "github.com/openshift/api/image/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	quotaName   = "isquota"
	waitTimeout = time.Second * 600
)

var _ = g.Describe("[sig-imageregistry][Feature:ImageQuota] Image resource quota", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("resourcequota-admission")

	g.It(fmt.Sprintf("should deny a push of built image exceeding %s quota [apigroup:image.openshift.io]", imagev1.ResourceImageStreams), g.Label("Size:M"), func() {
		g.Skip("TODO: determine why this test is not skipped/fails on 4.0 clusters")
		quota := corev1.ResourceList{
			imagev1.ResourceImageStreams: resource.MustParse("0"),
		}
		_, err := createResourceQuota(oc, quota)
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err := waitForResourceQuotaSync(oc, quotaName, quota)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, quota)).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image exceeding quota %v", quota))
		err = createImageStreamMapping(oc, oc.Namespace(), "first", "refused")
		assertQuotaExceeded(err)

		quota, err = bumpQuota(oc, imagev1.ResourceImageStreams, 1)
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

		quota, err = bumpQuota(oc, imagev1.ResourceImageStreams, 2)
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
		err = oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()).Delete(context.Background(), "first", metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err = exutil.WaitForResourceQuotaSync(
			oc.KubeClient().CoreV1().ResourceQuotas(oc.Namespace()),
			quotaName,
			corev1.ResourceList{imagev1.ResourceImageStreams: resource.MustParse("1")},
			true,
			waitTimeout,
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, corev1.ResourceList{imagev1.ResourceImageStreams: resource.MustParse("1")})).NotTo(o.HaveOccurred())

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
func createResourceQuota(oc *exutil.CLI, hard corev1.ResourceList) (*corev1.ResourceQuota, error) {
	rq := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name: quotaName,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: hard,
		},
	}

	g.By(fmt.Sprintf("creating resource quota with a limit %v", hard))
	rq, err := oc.AdminKubeClient().CoreV1().ResourceQuotas(oc.Namespace()).Create(context.Background(), rq, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	err = waitForLimitSync(oc, hard)
	return rq, err
}

// assertQuotasEqual compares two quota sets and returns an error with proper description when they don't match
func assertQuotasEqual(a, b corev1.ResourceList) error {
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
func bumpQuota(oc *exutil.CLI, resourceName corev1.ResourceName, value int64) (corev1.ResourceList, error) {
	g.By(fmt.Sprintf("bump the quota to %s=%d", resourceName, value))
	rq, err := oc.AdminKubeClient().CoreV1().ResourceQuotas(oc.Namespace()).Get(context.Background(), quotaName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	rq.Spec.Hard[resourceName] = *resource.NewQuantity(value, resource.DecimalSI)
	_, err = oc.AdminKubeClient().CoreV1().ResourceQuotas(oc.Namespace()).Update(context.Background(), rq, metav1.UpdateOptions{})
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
func waitForResourceQuotaSync(oc *exutil.CLI, name string, expectedResources corev1.ResourceList) (corev1.ResourceList, error) {
	g.By(fmt.Sprintf("waiting for resource quota %s to get updated", name))
	used, err := exutil.WaitForResourceQuotaSync(
		oc.KubeClient().CoreV1().ResourceQuotas(oc.Namespace()),
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
func waitForLimitSync(oc *exutil.CLI, hardLimit corev1.ResourceList) error {
	g.By(fmt.Sprintf("waiting for resource quota %s to get updated", quotaName))
	return waitForResourceQuotaLimitSync(
		oc.KubeClient().CoreV1().ResourceQuotas(oc.Namespace()),
		quotaName,
		hardLimit,
		waitTimeout)
}

// waitForResourceQuotaLimitSync watches given resource quota until its hard limit is updated to match the desired
// spec or timeout occurs.
func waitForResourceQuotaLimitSync(
	client corev1client.ResourceQuotaInterface,
	name string,
	hardLimit corev1.ResourceList,
	timeout time.Duration,
) error {
	startTime := time.Now()
	endTime := startTime.Add(timeout)

	expectedResourceNames := quota.ResourceNames(hardLimit)

	list, err := client.List(context.Background(), metav1.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector().String()})
	if err != nil {
		return err
	}

	for i := range list.Items {
		used := quota.Mask(list.Items[i].Status.Hard, expectedResourceNames)
		if isLimitSynced(used, hardLimit) {
			return nil
		}
	}

	rv := list.ResourceVersion
	w, err := client.Watch(context.Background(), metav1.ListOptions{FieldSelector: fields.Set{"metadata.name": name}.AsSelector().String(), ResourceVersion: rv})
	if err != nil {
		return err
	}
	defer w.Stop()

	for time.Now().Before(endTime) {
		select {
		case val, ok := <-w.ResultChan():
			if !ok {
				// reget and re-watch
				continue
			}
			if rq, ok := val.Object.(*corev1.ResourceQuota); ok {
				used := quota.Mask(rq.Status.Hard, expectedResourceNames)
				if isLimitSynced(used, hardLimit) {
					return nil
				}
			}
		case <-time.After(endTime.Sub(time.Now())):
			return wait.ErrWaitTimeout
		}
	}
	return wait.ErrWaitTimeout
}

func isLimitSynced(received, expected corev1.ResourceList) bool {
	resourceNames := quota.ResourceNames(expected)
	masked := quota.Mask(received, resourceNames)
	if len(masked) != len(expected) {
		return false
	}
	if le, _ := quota.LessThanOrEqual(masked, expected); !le {
		return false
	}
	if le, _ := quota.LessThanOrEqual(expected, masked); !le {
		return false
	}
	return true
}

func createImageStreamMapping(oc *exutil.CLI, namespace, name, tag string) error {
	e2e.Logf("Creating image stream mapping for %s/%s:%s...", namespace, name, tag)
	_, err := oc.AdminImageClient().ImageV1().ImageStreams(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if kerrors.IsNotFound(err) {
		_, err = oc.AdminImageClient().ImageV1().ImageStreams(namespace).Create(context.Background(), &imagev1.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	_, err = oc.AdminImageClient().ImageV1().ImageStreamMappings(namespace).Create(context.Background(), &imagev1.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Image: imagev1.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
		},
		Tag: tag,
	}, metav1.CreateOptions{})
	return err
}

func assertQuotaExceeded(err error) {
	o.Expect(kerrors.ReasonForError(err)).To(o.Equal(metav1.StatusReasonForbidden))
	o.Expect(err.Error()).To(o.ContainSubstring("exceeded quota"))
}
