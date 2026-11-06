package quota

import (
	"context"
	"fmt"
	"sort"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func integratedRegistryImportFrom(registryHost, namespace, imageStream string) string {
	return fmt.Sprintf("%s/%s/%s", registryHost, namespace, imageStream)
}

var _ = g.Describe("[sig-api-machinery][Feature:ResourceQuota]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("object-count-rq")

	g.Describe("Object count", func() {
		g.It(fmt.Sprintf("should properly count the number of imagestreams resources [apigroup:image.openshift.io]"), func() {
			clusterAdminKubeClient := oc.AdminKubeClient()
			clusterAdminImageClient := oc.AdminImageClient().ImageV1()
			testProject := oc.SetupProject()
			testResourceQuotaName := "count-imagestreams"

			rq := &corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{Name: testResourceQuotaName, Namespace: testProject},
				Spec: corev1.ResourceQuotaSpec{
					Hard: corev1.ResourceList{
						"openshift.io/imagestreams": resource.MustParse("10"),
					},
				},
			}

			_, err := clusterAdminKubeClient.CoreV1().ResourceQuotas(testProject).Create(context.Background(), rq, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForResourceQuotaStatus(clusterAdminKubeClient, testResourceQuotaName, testProject, func(actualResourceQuota *corev1.ResourceQuota) error {
				if !equality.Semantic.DeepEqual(actualResourceQuota.Spec.Hard, actualResourceQuota.Status.Hard) {
					return fmt.Errorf("%#v != %#v", actualResourceQuota.Spec.Hard, actualResourceQuota.Status.Hard)
				}
				expectedUsedStatus := corev1.ResourceList{
					"openshift.io/imagestreams": resource.MustParse("0"),
				}
				if !equality.Semantic.DeepEqual(actualResourceQuota.Status.Used, expectedUsedStatus) {
					return fmt.Errorf("unexpected current total usage: actul: %#v, expected: %#v", actualResourceQuota.Status.Used, expectedUsedStatus)
				}
				return nil
			})

			g.By("creating an image stream and checking the usage")
			imageStream := &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "empty-is"},
			}
			_, err = clusterAdminImageClient.ImageStreams(testProject).Create(context.Background(), imageStream, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitForResourceQuotaStatus(clusterAdminKubeClient, testResourceQuotaName, testProject, func(actualResourceQuota *corev1.ResourceQuota) error {
				expectedUsedStatus := corev1.ResourceList{
					"openshift.io/imagestreams": resource.MustParse("1"),
				}
				if !equality.Semantic.DeepEqual(actualResourceQuota.Status.Used, expectedUsedStatus) {
					return fmt.Errorf("unexpected current total usage: actual: %#v, expected: %#v", actualResourceQuota.Status.Used, expectedUsedStatus)
				}
				return nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deleting the image stream and checking the usage")
			err = clusterAdminImageClient.ImageStreams(testProject).Delete(context.Background(), imageStream.Name, metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitForResourceQuotaStatus(clusterAdminKubeClient, testResourceQuotaName, testProject, func(actualResourceQuota *corev1.ResourceQuota) error {
				expectedUsedStatus := corev1.ResourceList{
					"openshift.io/imagestreams": resource.MustParse("0"),
				}
				if !equality.Semantic.DeepEqual(actualResourceQuota.Status.Used, expectedUsedStatus) {
					return fmt.Errorf("unexpected current total usage: actual: %v, expected: %v", actualResourceQuota.Status.Used, expectedUsedStatus)
				}
				return nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should properly count the number of persistentvolumeclaims resources [Serial]", func() {
			testProject := oc.SetupProject()
			testResourceQuotaName := "my-resource-quota-" + testProject
			pvcName := "myclaim-" + testProject
			clusterAdminKubeClient := oc.AdminKubeClient()

			rq := &corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{Name: testResourceQuotaName, Namespace: testProject},
				Spec: corev1.ResourceQuotaSpec{
					Hard: corev1.ResourceList{
						"persistentvolumeclaims": resource.MustParse("1"),
					},
				},
			}

			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: pvcName,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("3Gi"),
						},
					},
				},
			}

			g.By("create the persistent volume and checking the usage")
			_, err := clusterAdminKubeClient.CoreV1().ResourceQuotas(testProject).Create(context.Background(), rq, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			err = waitForResourceQuotaStatus(clusterAdminKubeClient, testResourceQuotaName, testProject, func(actualResourceQuota *corev1.ResourceQuota) error {
				expectedUsedStatus := corev1.ResourceList{
					"persistentvolumeclaims": resource.MustParse("0"),
				}
				if !equality.Semantic.DeepEqual(actualResourceQuota.Status.Used, expectedUsedStatus) {
					return fmt.Errorf("unexpected current total usage: actual: %#v, expected: %#v", actualResourceQuota.Status.Used, expectedUsedStatus)
				}
				return nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = clusterAdminKubeClient.CoreV1().PersistentVolumeClaims(testProject).Create(context.Background(), pvc, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			err = waitForResourceQuotaStatus(clusterAdminKubeClient, testResourceQuotaName, testProject, func(actualResourceQuota *corev1.ResourceQuota) error {
				expectedUsedStatus := corev1.ResourceList{
					"persistentvolumeclaims": resource.MustParse("1"),
				}
				if !equality.Semantic.DeepEqual(actualResourceQuota.Status.Used, expectedUsedStatus) {
					return fmt.Errorf("unexpected current total usage: actual: %v, expected: %v", actualResourceQuota.Status.Used, expectedUsedStatus)
				}
				return nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = clusterAdminKubeClient.CoreV1().PersistentVolumeClaims(testProject).Create(context.Background(), pvc, metav1.CreateOptions{})
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.MatchRegexp(pvcName + `.*forbidden.*[Ee]xceeded quota`))

			g.By("deleting the persistent volume and checking the usage")
			err = clusterAdminKubeClient.CoreV1().PersistentVolumeClaims(testProject).Delete(context.Background(), pvcName, metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitForResourceQuotaStatus(clusterAdminKubeClient, testResourceQuotaName, testProject, func(actualResourceQuota *corev1.ResourceQuota) error {
				expectedUsedStatus := corev1.ResourceList{
					"persistentvolumeclaims": resource.MustParse("0"),
				}
				if !equality.Semantic.DeepEqual(actualResourceQuota.Status.Used, expectedUsedStatus) {
					return fmt.Errorf("unexpected current total usage: actual: %v, expected: %v", actualResourceQuota.Status.Used, expectedUsedStatus)
				}
				return nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("check the quota after import-image with --all option", func() {
			ctx := context.Background()
			testProject := oc.SetupProject()
			testResourceQuotaName := "my-imagestream-quota-" + testProject
			clusterAdminKubeClient := oc.AdminKubeClient()

			rq := &corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{Name: testResourceQuotaName, Namespace: testProject},
				Spec: corev1.ResourceQuotaSpec{
					Hard: corev1.ResourceList{
						"openshift.io/imagestreams": resource.MustParse("10"),
					},
				},
			}

			g.By("create the imagestreams and checking the usage")
			_, err := clusterAdminKubeClient.CoreV1().ResourceQuotas(testProject).Create(context.Background(), rq, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			err = waitForResourceQuotaStatus(clusterAdminKubeClient, testResourceQuotaName, testProject, func(actualResourceQuota *corev1.ResourceQuota) error {
				expectedUsedStatus := corev1.ResourceList{
					"openshift.io/imagestreams": resource.MustParse("0"),
				}
				if !equality.Semantic.DeepEqual(actualResourceQuota.Status.Used, expectedUsedStatus) {
					return fmt.Errorf("unexpected current total usage: actual: %#v, expected: %#v", actualResourceQuota.Status.Used, expectedUsedStatus)
				}
				return nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting until the integrated registry hostname is published")
			registryHost, err := exutil.WaitForInternalRegistryHostname(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			// Build a multi-tag ImageStream to expose it as one repository with several tags in the integrated registry.
			sourceISName := "rq-local-multi-src"
			sourceTags := []string{"alpha", "beta", "gamma"}
			for _, tag := range sourceTags {
				err = oc.AsAdmin().WithoutNamespace().Run("tag").Args(
					"openshift/cli:latest",
					fmt.Sprintf("%s:%s", sourceISName, tag),
					"-n", testProject,
				).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = exutil.WaitForAnImageStreamTag(oc, testProject, sourceISName, tag)
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("checking quota after creating one local ImageStream with multiple tags")
			err = waitForResourceQuotaStatus(clusterAdminKubeClient, testResourceQuotaName, testProject, func(actualResourceQuota *corev1.ResourceQuota) error {
				expectedUsedStatus := corev1.ResourceList{
					"openshift.io/imagestreams": resource.MustParse("1"),
				}
				if !equality.Semantic.DeepEqual(actualResourceQuota.Status.Used, expectedUsedStatus) {
					return fmt.Errorf("unexpected current total usage: actual: %#v, expected: %#v", actualResourceQuota.Status.Used, expectedUsedStatus)
				}
				return nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			sourceIS, err := oc.AdminImageClient().ImageV1().ImageStreams(testProject).Get(ctx, sourceISName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(len(sourceIS.Spec.Tags)).To(o.Equal(len(sourceTags)))

			bulkISName := "rq-bulk-import-is"
			importFrom := integratedRegistryImportFrom(registryHost, testProject, sourceISName)

			g.By("importing all tags from the local repository into one new ImageStream (bulk import adds exactly one ImageStream)")
			err = oc.AsAdmin().WithoutNamespace().Run("import-image").Args(
				bulkISName,
				"--from="+importFrom,
				"--confirm=true",
				"--all=true",
				"--request-timeout=5m",
				"-n", testProject,
			).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			var imported *imagev1.ImageStream
			err = utilwait.PollUntilContextTimeout(ctx, 1*time.Second, 1*time.Minute, true, func(ctx context.Context) (bool, error) {
				var getErr error
				imported, getErr = oc.AdminImageClient().ImageV1().ImageStreams(testProject).Get(ctx, bulkISName, metav1.GetOptions{})
				if getErr != nil {
					return false, nil
				}
				return len(imported.Spec.Tags) == len(sourceTags), nil
			})

			o.Expect(err).NotTo(o.HaveOccurred(),
				"import-image --all should copy multiple tags onto a single ImageStream (timed out waiting for tags to populate)")
			g.By("checking that bulk import increased ImageStream quota by exactly one")
			err = waitForResourceQuotaStatus(clusterAdminKubeClient, testResourceQuotaName, testProject, func(actualResourceQuota *corev1.ResourceQuota) error {
				expectedUsedStatus := corev1.ResourceList{
					"openshift.io/imagestreams": resource.MustParse("2"),
				}
				if !equality.Semantic.DeepEqual(actualResourceQuota.Status.Used, expectedUsedStatus) {
					return fmt.Errorf("unexpected current total usage: actual: %#v, expected: %#v", actualResourceQuota.Status.Used, expectedUsedStatus)
				}
				return nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("tagging another ImageStream from an in-cluster ImageStreamTag")
			err = oc.AsAdmin().WithoutNamespace().Run("tag").Args(
				"openshift/cli:latest",
				"mystream:latest",
				"-n", testProject,
			).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = exutil.WaitForAnImageStreamTag(oc, testProject, "mystream", "latest")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("checking the imagestream usage again")
			err = waitForResourceQuotaStatus(clusterAdminKubeClient, testResourceQuotaName, testProject, func(actualResourceQuota *corev1.ResourceQuota) error {
				expectedUsedStatus := corev1.ResourceList{
					"openshift.io/imagestreams": resource.MustParse("3"),
				}
				if !equality.Semantic.DeepEqual(actualResourceQuota.Status.Used, expectedUsedStatus) {
					return fmt.Errorf("unexpected current total usage: actual: %#v, expected: %#v", actualResourceQuota.Status.Used, expectedUsedStatus)
				}
				return nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("when exceed openshift.io/image-tags will ban to create new image references in the project", func() {
			testProject := oc.SetupProject()
			testResourceQuotaName := "my-image-tag-quota"
			clusterAdminKubeClient := oc.AdminKubeClient()

			lr := &corev1.LimitRange{
				ObjectMeta: metav1.ObjectMeta{Name: testResourceQuotaName, Namespace: testProject},
				Spec: corev1.LimitRangeSpec{
					Limits: []corev1.LimitRangeItem{
						{
							Type: "openshift.io/ImageStream",
							Max: corev1.ResourceList{
								"openshift.io/image-tags": resource.MustParse("2"),
							},
						},
					},
				},
			}

			g.By("create the image-tags and checking the usage")
			_, err := clusterAdminKubeClient.CoreV1().LimitRanges(testProject).Create(context.Background(), lr, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			err = waitForLimitRangeStatus(clusterAdminKubeClient, testResourceQuotaName, testProject, func(actualLimitRange *corev1.LimitRange) error {
				expectedLimitRange := []corev1.LimitRangeItem{
					{
						Type: "openshift.io/ImageStream",
						Max: corev1.ResourceList{
							"openshift.io/image-tags": resource.MustParse("2"),
						},
					},
				}
				// Compare actual and expected LimitRangeItems
				if !equality.Semantic.DeepEqual(actualLimitRange.Spec.Limits, expectedLimitRange) {
					return fmt.Errorf("unexpected current total usage: actual: %#v, expected: %#v", actualLimitRange.Spec.Limits, expectedLimitRange)
				}
				return nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			sourceISTags, ok := pickOpenshiftSourceISTagsForTagQuota(oc)
			if !ok {
				g.Skip("need three openshift ImageStreams with at least one resolved tag each for oc tag --source=istag; skipping image-tag quota enforcement check")
			}

			tags := []string{"v1", "v2", "v3"}
			for i := range 2 {
				g.By("trying to tag a container image with " + tags[i] + " from " + sourceISTags[i])
				err = oc.Run("tag").Args(sourceISTags[i], "--source=istag", "mystream:"+tags[i], "-n", testProject).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = exutil.WaitForAnImageStreamTag(oc, testProject, "mystream", tags[i])
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("waiting until mystream records both tags so limit enforcement sees tag count before v3")
			err = waitForImageStreamStatusTagsPopulated(oc, testProject, "mystream", tags[:2])
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("trying to tag a container image with v3 from " + sourceISTags[2])
			err = oc.Run("tag").Args(sourceISTags[2], "--source=istag", "mystream:v3", "-n", testProject).Execute()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(err.Error()).To(o.MatchRegexp(`.*forbidden.*[Ee]xceed`))
		})
	})
})

func pickOpenshiftSourceISTagsForTagQuota(oc *exutil.CLI) ([]string, bool) {
	const ns = "openshift"
	list, err := oc.AdminImageClient().ImageV1().ImageStreams(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, false
	}
	sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].Name < list.Items[j].Name })

	var refs []string
	for i := range list.Items {
		is := &list.Items[i]
		if len(is.Spec.Tags) == 0 {
			continue
		}
		names := make([]string, len(is.Spec.Tags))
		for j := range is.Spec.Tags {
			names[j] = is.Spec.Tags[j].Name
		}
		sort.Slice(names, func(i, j int) bool {
			a, b := names[i], names[j]
			aLatest, bLatest := a == imagev1.DefaultImageTag, b == imagev1.DefaultImageTag
			if aLatest != bLatest {
				return aLatest
			}
			return a < b
		})
		tag := ""
		for _, n := range names {
			ev, ok := imageutil.StatusHasTag(is, n)
			if ok && len(ev.Items) > 0 {
				tag = n
				break
			}
		}
		if tag == "" {
			continue
		}
		refs = append(refs, fmt.Sprintf("%s/%s:%s", ns, is.Name, tag))
		if len(refs) == 3 {
			return refs, true
		}
	}
	return nil, false
}

func waitForImageStreamStatusTagsPopulated(oc *exutil.CLI, namespace, name string, tags []string) error {
	streams := oc.AdminImageClient().ImageV1().ImageStreams(namespace)
	return utilwait.PollImmediate(200*time.Millisecond, 2*time.Minute, func() (bool, error) {
		is, err := streams.Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		for _, t := range tags {
			st, ok := imageutil.StatusHasTag(is, t)
			if !ok || len(st.Items) == 0 {
				return false, nil
			}
		}
		return true, nil
	})
}

func waitForResourceQuotaStatus(clusterAdminKubeClient kubernetes.Interface, name string, namespace string, conditionFn func(*corev1.ResourceQuota) error) error {
	var pollErr error
	err := utilwait.PollUntilContextTimeout(context.Background(), 100*time.Millisecond, QuotaWaitTimeout, true, func(ctx context.Context) (done bool, err error) {
		quota, err := clusterAdminKubeClient.CoreV1().ResourceQuotas(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			pollErr = err
			return false, nil
		}
		err = conditionFn(quota)
		if err == nil {
			return true, nil
		}
		pollErr = err
		return false, nil
	})
	if err != nil {
		err = fmt.Errorf("%s: %s", err, pollErr)
	}
	return err
}

func waitForLimitRangeStatus(clusterAdminKubeClient kubernetes.Interface, name string, namespace string, conditionFn func(*corev1.LimitRange) error) error {
	var pollErr error
	err := utilwait.PollImmediate(100*time.Millisecond, QuotaWaitTimeout, func() (done bool, err error) {
		limitRange, err := clusterAdminKubeClient.CoreV1().LimitRanges(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			pollErr = err
			return false, nil
		}
		err = conditionFn(limitRange)
		if err == nil {
			return true, nil
		}
		pollErr = err
		return false, nil
	})
	if err != nil {
		err = fmt.Errorf("%s: %s", err, pollErr)
	}
	return err
}
