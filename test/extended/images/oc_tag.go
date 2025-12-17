package images

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	k8simage "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"

	authorizationv1 "github.com/openshift/api/authorization/v1"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[sig-imageregistry][Feature:Image] oc tag", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithPodSecurityLevel("image-oc-tag", admissionapi.LevelBaseline)
	ctx := context.Background()

	g.It("should preserve image reference for external images [apigroup:image.openshift.io]", g.Label("Size:S"), func() {
		var (
			externalImage = k8simage.GetE2EImage(k8simage.BusyBox)
			isName        = "busybox"
			isName2       = "busybox2"
		)

		externalRepository := externalImage
		if i := strings.LastIndex(externalRepository, ":"); i != -1 {
			externalRepository = externalRepository[:i]
		}

		g.By("import an external image")

		err := oc.Run("tag").Args("--source=docker", externalImage, isName+":latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), isName, "latest")
		o.Expect(err).NotTo(o.HaveOccurred())

		// check that the created image stream references the external registry
		is, err := oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()).Get(ctx, isName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(is.Status.Tags).To(o.HaveLen(1))
		tag1 := is.Status.Tags[0]
		o.Expect(tag1.Tag).To(o.Equal("latest"))
		o.Expect(tag1.Items).To(o.HaveLen(1))
		o.Expect(tag1.Items[0].DockerImageReference).To(o.HavePrefix(externalRepository + "@"))

		g.By("copy the image to another image stream")

		err = oc.Run("tag").Args("--source=istag", isName+":latest", isName2+":latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), isName2, "latest")
		o.Expect(err).NotTo(o.HaveOccurred())

		// check that the new image stream references the still uses the external registry
		is, err = oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()).Get(ctx, isName2, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(is.Status.Tags).To(o.HaveLen(1))
		tag2 := is.Status.Tags[0]
		o.Expect(tag2.Tag).To(o.Equal("latest"))
		o.Expect(tag2.Items).To(o.HaveLen(1))
		o.Expect(tag2.Items[0].DockerImageReference).To(o.Equal(tag1.Items[0].DockerImageReference))
	})

	g.It("should change image reference for internal images [apigroup:build.openshift.io][apigroup:image.openshift.io]", g.Label("Size:M"), func() {
		var (
			isName     = "localimage"
			isName2    = "localimage2"
			dockerfile = fmt.Sprintf(`FROM %s
RUN touch /test-image
`, image.ShellImage())
		)

		g.By("determine the name of the integrated registry")

		registryHost, _, err := oc.Run("registry").Args("info", "--internal").Outputs()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("build an image")

		err = oc.Run("new-build").Args("-D", "-", "--to", isName+":latest").InputString(dockerfile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), isName+"-1", nil, nil, nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		// check that the created image stream references the integrated registry
		is, err := oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()).Get(ctx, isName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(is.Status.Tags).To(o.HaveLen(1))
		tag := is.Status.Tags[0]
		o.Expect(tag.Tag).To(o.Equal("latest"))
		o.Expect(tag.Items).To(o.HaveLen(1))
		o.Expect(tag.Items[0].DockerImageReference).To(o.HavePrefix(fmt.Sprintf("%s/%s/%s@", registryHost, oc.Namespace(), isName)))

		// extract the image digest
		ref := tag.Items[0].DockerImageReference
		digest := ref[strings.Index(ref, "@")+1:]
		o.Expect(digest).To(o.HavePrefix("sha256:"))

		g.By("copy the image to another image stream")

		err = oc.Run("tag").Args("--source=istag", isName+":latest", isName2+":latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), isName2, "latest")
		o.Expect(err).NotTo(o.HaveOccurred())

		// check that the new image stream uses its own name in the image reference
		is, err = oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()).Get(ctx, isName2, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(is.Status.Tags).To(o.HaveLen(1))
		tag = is.Status.Tags[0]
		o.Expect(tag.Tag).To(o.Equal("latest"))
		o.Expect(tag.Items).To(o.HaveLen(1))
		o.Expect(tag.Items[0].DockerImageReference).To(o.Equal(fmt.Sprintf("%s/%s/%s@%s", registryHost, oc.Namespace(), isName2, digest)))
	})

	g.It("should work when only imagestreams api is available [apigroup:image.openshift.io][apigroup:authorization.openshift.io]", g.Label("Size:S"), func() {
		err := oc.Run("tag").Args("--source=docker", image.ShellImage(), "testis:latest").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "testis", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("create").Args("serviceaccount", "testsa").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Creating a role that allows to work with imagestreams, but not imagestreamtags...")

		_, err = oc.AdminAuthorizationClient().AuthorizationV1().Roles(oc.Namespace()).Create(ctx, &authorizationv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testrole",
			},
			Rules: []authorizationv1.PolicyRule{
				{
					Verbs:     []string{"get", "update"},
					APIGroups: []string{"image.openshift.io"},
					Resources: []string{"imagestreams"},
				},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("policy").Args("add-role-to-user", "testrole", "-z", "testsa", "--role-namespace="+oc.Namespace()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		token, err := oc.Run("create").Args("token", "testsa").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("login").Args("--token=" + token).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("whoami").Args().Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("tag").Args("testis:latest", "testis:copy").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		e2e.Logf("Checking that the imagestream is updated...")

		is, err := oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()).Get(ctx, "testis", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		var tags []string
		for _, t := range is.Spec.Tags {
			tags = append(tags, t.Name)
		}
		o.Expect(tags).To(o.ContainElement("copy"), "testis spec.tags should contain the tag copy")
	})
})
