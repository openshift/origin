package imageapis

import (
	"fmt"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	kutilerrors "k8s.io/kubernetes/pkg/util/errors"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	imageapi "github.com/openshift/origin/pkg/image/api"
	imagesutil "github.com/openshift/origin/test/extended/images"
	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
)

const (
	imageSize = 100

	quotaName = "isquota"

	waitTimeout = time.Second * 30
)

var _ = g.Describe("[imageapis] openshift resource quota admission", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("resourcequota-admission", exutil.KubeConfigPath())

	g.JustBeforeEach(func() {
		g.By("Waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	// needs to be run at the of of each It; cannot be run in AfterEach which is run after the project
	// is destroyed
	tearDown := func(oc *exutil.CLI) {
		g.By(fmt.Sprintf("Deleting quota %s", quotaName))
		oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Delete(quotaName)

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
		_, err = imagesutil.BuildAndPushImageOfSizeWithDocker(oc, dClient, "first", "refused", imageSize, 1, outSink, false)
		o.Expect(err).NotTo(o.HaveOccurred())

		quota, err = bumpQuota(oc, imageapi.ResourceImageStreams, 1)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image below quota %v", quota))
		_, err = imagesutil.BuildAndPushImageOfSizeWithDocker(oc, dClient, "first", "tag1", imageSize, 1, outSink, true)
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err := waitForResourceQuotaSync(oc, quotaName, quota)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, quota)).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image to existing image stream %v", quota))
		_, err = imagesutil.BuildAndPushImageOfSizeWithDocker(oc, dClient, "first", "tag2", imageSize, 1, outSink, true)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image exceeding quota %v", quota))
		_, err = imagesutil.BuildAndPushImageOfSizeWithDocker(oc, dClient, "second", "refused", imageSize, 1, outSink, false)

		quota, err = bumpQuota(oc, imageapi.ResourceImageStreams, 2)
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err = waitForResourceQuotaSync(oc, quotaName, used)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image below quota %v", quota))
		_, err = imagesutil.BuildAndPushImageOfSizeWithDocker(oc, dClient, "second", "tag1", imageSize, 1, outSink, true)
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err = waitForResourceQuotaSync(oc, quotaName, quota)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, quota)).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image exceeding quota %v", quota))
		_, err = imagesutil.BuildAndPushImageOfSizeWithDocker(oc, dClient, "third", "refused", imageSize, 1, outSink, false)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("deleting first image stream")
		err = oc.REST().ImageStreams(oc.Namespace()).Delete("first")
		o.Expect(err).NotTo(o.HaveOccurred())
		used, err = exutil.WaitForResourceQuotaSync(
			oc.KubeREST().ResourceQuotas(oc.Namespace()),
			quotaName,
			kapi.ResourceList{imageapi.ResourceImageStreams: resource.MustParse("1")},
			true,
			waitTimeout,
		)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(assertQuotasEqual(used, kapi.ResourceList{imageapi.ResourceImageStreams: resource.MustParse("1")})).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("trying to push image below quota %v", quota))
		_, err = imagesutil.BuildAndPushImageOfSizeWithDocker(oc, dClient, "third", "tag", imageSize, 1, outSink, true)
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
		ObjectMeta: kapi.ObjectMeta{
			Name: quotaName,
		},
		Spec: kapi.ResourceQuotaSpec{
			Hard: hard,
		},
	}

	g.By(fmt.Sprintf("creating resource quota with a limit %v", hard))
	rq, err := oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Create(rq)
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
	rq, err := oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Get(quotaName)
	if err != nil {
		return nil, err
	}
	rq.Spec.Hard[resourceName] = *resource.NewQuantity(value, resource.DecimalSI)
	_, err = oc.AdminKubeREST().ResourceQuotas(oc.Namespace()).Update(rq)
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
		oc.KubeREST().ResourceQuotas(oc.Namespace()),
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
		oc.KubeREST().ResourceQuotas(oc.Namespace()),
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
		iss, err := oc.AdminREST().ImageStreams(projectName).List(kapi.ListOptions{})
		if err != nil {
			continue
		}
		for _, is := range iss.Items {
			for _, history := range is.Status.Tags {
				for i := range history.Items {
					oc.AdminREST().Images().Delete(history.Items[i].Image)
				}
			}
			for _, tagRef := range is.Spec.Tags {
				switch tagRef.From.Kind {
				case "ImageStreamImage":
					_, id, err := imageapi.ParseImageStreamImageName(tagRef.From.Name)
					if err != nil {
						continue
					}
					oc.AdminREST().Images().Delete(id)
				}
			}
		}

		// let the extended framework take care of the current namespace
		if projectName != oc.Namespace() {
			g.By(fmt.Sprintf("Deleting project %q", projectName))
			oc.AdminREST().Projects().Delete(projectName)
		}
	}
}
