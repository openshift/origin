package imageapis

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imagesutil "github.com/openshift/origin/test/extended/images"
	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
)

const (
	imageSize = 100

	quotaName = "isquota"

	waitTimeout = time.Second * 600
)

var _ = g.Describe("[Feature:ImageQuota][registry][Serial] Image resource quota", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("resourcequota-admission", exutil.KubeConfigPath())

	g.JustBeforeEach(func() {
		g.By("Waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// needs to be run at the of of each It; cannot be run in AfterEach which is run after the project
	// is destroyed
	tearDown := func(oc *exutil.CLI) {
		g.By(fmt.Sprintf("Deleting quota %s", quotaName))
		oc.AdminKubeClient().Core().ResourceQuotas(oc.Namespace()).Delete(quotaName, nil)

		deleteTestImagesAndStreams(oc)
	}

	g.It(fmt.Sprintf("should deny a push of built image exceeding %s quota", imageapi.ResourceImageStreams), func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)
		defer tearDown(oc)
		dClient, err := testutil.NewDockerClient()
		o.Expect(err).NotTo(o.HaveOccurred())

		outSink := g.GinkgoWriter

		quota := kapi.ResourceList{
			imageapi.ResourceImageStreams: resource.MustParse("0"),
		}
		_, err = createResourceQuota(oc, quota)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image exceeding quota %v", quota))
		_, _, err = imagesutil.BuildAndPushImageOfSizeWithDocker(oc, dClient, "first", "refused", imageSize, 1, outSink, false, true)
		o.Expect(err).NotTo(o.HaveOccurred())

		quota, err = bumpQuota(oc, imageapi.ResourceImageStreams, 1)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image below quota %v", quota))
		_, _, err = imagesutil.BuildAndPushImageOfSizeWithDocker(oc, dClient, "first", "tag1", imageSize, 1, outSink, true, true)
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err := waitForResourceQuotaSync(oc, quotaName, quota)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, quota)).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image to existing image stream %v", quota))
		_, _, err = imagesutil.BuildAndPushImageOfSizeWithDocker(oc, dClient, "first", "tag2", imageSize, 1, outSink, true, true)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image exceeding quota %v", quota))
		_, _, err = imagesutil.BuildAndPushImageOfSizeWithDocker(oc, dClient, "second", "refused", imageSize, 1, outSink, false, true)

		quota, err = bumpQuota(oc, imageapi.ResourceImageStreams, 2)
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err = waitForResourceQuotaSync(oc, quotaName, used)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image below quota %v", quota))
		_, _, err = imagesutil.BuildAndPushImageOfSizeWithDocker(oc, dClient, "second", "tag1", imageSize, 1, outSink, true, true)
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err = waitForResourceQuotaSync(oc, quotaName, quota)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, quota)).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image exceeding quota %v", quota))
		_, _, err = imagesutil.BuildAndPushImageOfSizeWithDocker(oc, dClient, "third", "refused", imageSize, 1, outSink, false, true)
		o.Expect(err).NotTo(o.HaveOccurred())

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
		_, _, err = imagesutil.BuildAndPushImageOfSizeWithDocker(oc, dClient, "third", "tag", imageSize, 1, outSink, true, true)
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

// deleteTestImagesAndStreams deletes test images built in current and shared
// namespaces. It also deletes shared projects.
func deleteTestImagesAndStreams(oc *exutil.CLI) {
	for _, projectName := range []string{
		oc.Namespace() + "-s2",
		oc.Namespace() + "-s1",
		oc.Namespace() + "-shared",
		oc.Namespace(),
	} {
		g.By(fmt.Sprintf("Deleting images and image streams in project %q", projectName))
		iss, err := oc.AdminImageClient().Image().ImageStreams(projectName).List(metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, is := range iss.Items {
			for _, history := range is.Status.Tags {
				for i := range history.Items {
					oc.AdminImageClient().Image().Images().Delete(history.Items[i].Image, nil)
				}
			}
			for _, tagRef := range is.Spec.Tags {
				switch tagRef.From.Kind {
				case "ImageStreamImage":
					_, id, err := imageapi.ParseImageStreamImageName(tagRef.From.Name)
					if err != nil {
						continue
					}
					oc.AdminImageClient().Image().Images().Delete(id, nil)
				}
			}
		}

		// let the extended framework take care of the current namespace
		if projectName != oc.Namespace() {
			g.By(fmt.Sprintf("Deleting project %q", projectName))
			oc.AdminProjectClient().Project().Projects().Delete(projectName, nil)
		}
	}
}
