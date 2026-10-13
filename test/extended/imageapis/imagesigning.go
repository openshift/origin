package imageapis

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"

	kubeauthorizationv1 "k8s.io/api/authorization/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"

	g "github.com/onsi/ginkgo/v2"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-imageregistry][Feature:Image] signature", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("image")

	g.It("TestImageAddSignature [apigroup:image.openshift.io]", func() {
		t := g.GinkgoT()

		adminClient := oc.AdminImageClient()

		image, err := getImageFixture(oc, exutil.FixturePath("testdata", "image", "test-image.json"))
		if err != nil {
			t.Fatalf("failed to read image fixture: %v", err)
		}

		image, err = adminClient.ImageV1().Images().Create(context.Background(), image, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("unexpected error creating image: %v", err)
		}

		if len(image.Signatures) != 0 {
			t.Fatalf("expected empty signatures, not: %s", diff.Diff(image.Signatures, []imagev1.ImageSignature{}))
		}

		userClient := oc.ImageClient()

		if len(image.Signatures) != 0 {
			t.Fatalf("expected empty signatures, not: %s", diff.Diff(image.Signatures, []imagev1.ImageSignature{}))
		}

		// add some dummy signature
		signature := imagev1.ImageSignature{
			Type:    "unknown",
			Content: []byte("binaryblob"),
		}

		sigName, err := joinImageSignatureName(image.Name, "signaturename")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		signature.Name = sigName

		created, err := userClient.ImageV1().ImageSignatures().Create(context.Background(), &signature, metav1.CreateOptions{})
		if err == nil {
			t.Fatalf("unexpected success updating image signatures")
		}
		if !kerrors.IsForbidden(err) {
			t.Fatalf("expected forbidden error, not: %v", err)
		}

		makeUserAnImageSigner(oc)

		// try to create the signature again
		created, err = userClient.ImageV1().ImageSignatures().Create(context.Background(), &signature, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		image, err = adminClient.ImageV1().Images().Get(context.Background(), image.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(image.Signatures) != 1 {
			t.Fatalf("unexpected number of signatures in created image (%d != %d)", len(image.Signatures), 1)
		}
		for _, sig := range []*imagev1.ImageSignature{created, &image.Signatures[0]} {
			if sig.Name != sigName || sig.Type != "unknown" ||
				!bytes.Equal(sig.Content, []byte("binaryblob")) || len(sig.Conditions) != 0 {
				t.Errorf("unexpected signature received: %#+v", sig)
			}
		}
		compareSignatures(t, image.Signatures[0], *created)

		// try to create the signature yet again
		created, err = userClient.ImageV1().ImageSignatures().Create(context.Background(), &signature, metav1.CreateOptions{})
		if !kerrors.IsAlreadyExists(err) {
			t.Fatalf("expected already exists error, not: %v", err)
		}

		// try to create a signature with different name but the same conent
		newName, err := joinImageSignatureName(image.Name, "newone")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		signature.Name = newName
		created, err = userClient.ImageV1().ImageSignatures().Create(context.Background(), &signature, metav1.CreateOptions{})
		if !kerrors.IsAlreadyExists(err) {
			t.Fatalf("expected already exists error, not: %v", err)
		}

		// try to create a signature with the same name but different content
		signature.Name = sigName
		signature.Content = []byte("different")
		_, err = userClient.ImageV1().ImageSignatures().Create(context.Background(), &signature, metav1.CreateOptions{})
		if !kerrors.IsAlreadyExists(err) {
			t.Fatalf("expected already exists error, not: %v", err)
		}
	})
})

var _ = g.Describe("[sig-imageregistry][Feature:Image] signature", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("image").AsAdmin()
	ctx := context.Background()

	g.It("TestImageRemoveSignature [apigroup:image.openshift.io]", func() {
		t := g.GinkgoT()

		adminClient := oc.AdminImageClient()

		image, err := getImageFixture(oc, exutil.FixturePath("testdata", "image", "test-image.json"))
		if err != nil {
			t.Fatalf("failed to read image fixture: %v", err)
		}

		image, err = adminClient.ImageV1().Images().Create(ctx, image, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("unexpected error creating image: %v", err)
		}

		if len(image.Signatures) != 0 {
			t.Fatalf("expected empty signatures, not: %s", diff.Diff(image.Signatures, []imagev1.ImageSignature{}))
		}

		userClient := oc.ImageClient()

		makeUserAnImageSigner(oc)

		// create some signatures
		sigData := []struct {
			sigName string
			content string
		}{
			{"a", "binaryblob"},
			{"b", "security without obscurity"},
			{"c", "distrust and caution are the parents of security"},
			{"d", "he who sacrifices freedom for security deserves neither"},
		}
		for i, d := range sigData {
			name, err := joinImageSignatureName(image.Name, d.sigName)
			if err != nil {
				t.Fatalf("creating signature %d: unexpected error: %v", i, err)
			}
			signature := imagev1.ImageSignature{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Type:    "unknown",
				Content: []byte(d.content),
			}
			_, err = userClient.ImageV1().ImageSignatures().Create(ctx, &signature, metav1.CreateOptions{})
			if err != nil {
				t.Fatalf("creating signature %d: unexpected error: %v", i, err)
			}
		}

		image, err = userClient.ImageV1().Images().Get(ctx, image.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(image.Signatures) != 4 {
			t.Fatalf("expected 4 signatures, not %d", len(image.Signatures))
		}

		delOptions := metav1.DeleteOptions{}

		// try to delete blob that does not exist
		err = userClient.ImageV1().ImageSignatures().Delete(ctx, image.Name+"@doesnotexist", delOptions)
		if !kerrors.IsNotFound(err) {
			t.Fatalf("expected not found error, not: %#+v", err)
		}

		// try to delete blob with missing signature name
		err = userClient.ImageV1().ImageSignatures().Delete(ctx, image.Name+"@", delOptions)
		if !kerrors.IsBadRequest(err) {
			t.Fatalf("expected bad request, not: %#+v", err)
		}

		// delete the first
		err = userClient.ImageV1().ImageSignatures().Delete(ctx, image.Name+"@"+sigData[0].sigName, delOptions)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// try to delete it once more
		err = userClient.ImageV1().ImageSignatures().Delete(ctx, image.Name+"@"+sigData[0].sigName, delOptions)
		if err == nil {
			t.Fatalf("unexpected nont error")
		} else if !kerrors.IsNotFound(err) {
			t.Errorf("expected not found error, not: %#+v", err)
		}

		// delete the one in the middle
		err = userClient.ImageV1().ImageSignatures().Delete(ctx, image.Name+"@"+sigData[2].sigName, delOptions)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if image, err = userClient.ImageV1().Images().Get(ctx, image.Name, metav1.GetOptions{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		} else if len(image.Signatures) != 2 {
			t.Fatalf("expected 2 signatures, not %d", len(image.Signatures))
		}

		// delete the one at the end
		err = userClient.ImageV1().ImageSignatures().Delete(ctx, image.Name+"@"+sigData[3].sigName, delOptions)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// delete the last one
		err = userClient.ImageV1().ImageSignatures().Delete(ctx, image.Name+"@"+sigData[1].sigName, delOptions)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if image, err = userClient.ImageV1().Images().Get(ctx, image.Name, metav1.GetOptions{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		} else if len(image.Signatures) != 0 {
			t.Fatalf("expected 2 signatures, not %d", len(image.Signatures))
		}
	})
})

func getImageFixture(oc *exutil.CLI, filename string) (*imagev1.Image, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	obj, err := helpers.ReadYAML(bytes.NewBuffer(data), imagev1.Install)
	if err != nil {
		return nil, err
	}

	obj.(*imagev1.Image).Name = obj.(*imagev1.Image).Name + oc.Namespace()
	oc.AddResourceToDelete(imagev1.GroupVersion.WithResource("images"), obj.(*imagev1.Image))

	return obj.(*imagev1.Image), nil
}

func makeUserAnImageSigner(oc *exutil.CLI) error {
	rolebinding, err := oc.AdminKubeClient().RbacV1().ClusterRoleBindings().Create(context.Background(), &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "image-signer-" + oc.Namespace()},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "system:image-signer",
		},
		Subjects: []rbacv1.Subject{{Kind: "User", Name: oc.Username()}},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	oc.AddResourceToDelete(rbacv1.SchemeGroupVersion.WithResource("clusterroles"), rolebinding)

	return oc.WaitForAccessAllowed(&kubeauthorizationv1.SelfSubjectAccessReview{
		Spec: kubeauthorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
				Verb:     "create",
				Group:    "",
				Resource: "imagesignatures",
			},
		},
	}, oc.Username())
}

func compareSignatures(t g.GinkgoTInterface, a, b imagev1.ImageSignature) {
	aName := a.Name
	a.ObjectMeta = b.ObjectMeta
	a.Name = aName
	if !reflect.DeepEqual(a, b) {
		t.Errorf("created and contained signatures differ: %v", diff.Diff(a, b))
	}
}

// joinImageSignatureName joins image name and custom signature name into one string with @ separator.
func joinImageSignatureName(imageName, signatureName string) (string, error) {
	if len(imageName) == 0 {
		return "", fmt.Errorf("imageName may not be empty")
	}
	if len(signatureName) == 0 {
		return "", fmt.Errorf("signatureName may not be empty")
	}
	if strings.Count(imageName, "@") > 0 || strings.Count(signatureName, "@") > 0 {
		return "", fmt.Errorf("neither imageName nor signatureName can contain '@'")
	}
	return fmt.Sprintf("%s@%s", imageName, signatureName), nil
}
