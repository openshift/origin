package integration

import (
	"bytes"
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util/diff"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/admin/policy"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	imageapi "github.com/openshift/origin/pkg/image/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

const testUserName = "bob"

func TestImageAddSignature(t *testing.T) {
	adminClient, userClient, image := testSetupImageSignatureTest(t, testUserName)

	if len(image.Signatures) != 0 {
		t.Fatalf("expected empty signatures, not: %s", diff.ObjectDiff(image.Signatures, []imageapi.ImageSignature{}))
	}

	// add some dummy signature
	signature := imageapi.ImageSignature{
		Type:    "unknown",
		Content: []byte("binaryblob"),
	}

	sigName, err := imageapi.JoinImageSignatureName(image.Name, "signaturename")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	signature.Name = sigName

	created, err := userClient.ImageSignatures().Create(&signature)
	if err == nil {
		t.Fatalf("unexpected success updating image signatures")
	}
	if !kerrors.IsForbidden(err) {
		t.Fatalf("expected forbidden error, not: %v", err)
	}

	makeUserAnImageSigner(adminClient, userClient, testUserName)

	// try to create the signature again
	created, err = userClient.ImageSignatures().Create(&signature)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	image, err = adminClient.Images().Get(image.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(image.Signatures) != 1 {
		t.Fatalf("unexpected number of signatures in created image (%d != %d)", len(image.Signatures), 1)
	}
	for _, sig := range []*imageapi.ImageSignature{created, &image.Signatures[0]} {
		if sig.Name != sigName || sig.Type != "unknown" ||
			!bytes.Equal(sig.Content, []byte("binaryblob")) || len(sig.Conditions) != 0 {
			t.Errorf("unexpected signature received: %#+v", sig)
		}
	}
	compareSignatures(t, image.Signatures[0], *created)

	// try to create the signature yet again
	created, err = userClient.ImageSignatures().Create(&signature)
	if !kerrors.IsAlreadyExists(err) {
		t.Fatalf("expected already exists error, not: %v", err)
	}

	// try to create a signature with different name but the same conent
	newName, err := imageapi.JoinImageSignatureName(image.Name, "newone")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	signature.Name = newName
	created, err = userClient.ImageSignatures().Create(&signature)
	if !kerrors.IsAlreadyExists(err) {
		t.Fatalf("expected already exists error, not: %v", err)
	}

	// try to create a signature with the same name but different content
	signature.Name = sigName
	signature.Content = []byte("different")
	_, err = userClient.ImageSignatures().Create(&signature)
	if !kerrors.IsAlreadyExists(err) {
		t.Fatalf("expected already exists error, not: %v", err)
	}
}

func TestImageRemoveSignature(t *testing.T) {
	adminClient, userClient, image := testSetupImageSignatureTest(t, testUserName)
	makeUserAnImageSigner(adminClient, userClient, testUserName)

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
		name, err := imageapi.JoinImageSignatureName(image.Name, d.sigName)
		if err != nil {
			t.Fatalf("creating signature %d: unexpected error: %v", i, err)
		}
		signature := imageapi.ImageSignature{
			ObjectMeta: kapi.ObjectMeta{
				Name: name,
			},
			Type:    "unknown",
			Content: []byte(d.content),
		}
		_, err = userClient.ImageSignatures().Create(&signature)
		if err != nil {
			t.Fatalf("creating signature %d: unexpected error: %v", i, err)
		}
	}

	image, err := userClient.Images().Get(image.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(image.Signatures) != 4 {
		t.Fatalf("expected 4 signatures, not %d", len(image.Signatures))
	}

	// try to delete blob that does not exist
	err = userClient.ImageSignatures().Delete(image.Name + "@doesnotexist")
	if !kerrors.IsNotFound(err) {
		t.Fatalf("expected not found error, not: %#+v", err)
	}

	// try to delete blob with missing signature name
	err = userClient.ImageSignatures().Delete(image.Name + "@")
	if !kerrors.IsBadRequest(err) {
		t.Fatalf("expected bad request, not: %#+v", err)
	}

	// delete the first
	err = userClient.ImageSignatures().Delete(image.Name + "@" + sigData[0].sigName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// try to delete it once more
	err = userClient.ImageSignatures().Delete(image.Name + "@" + sigData[0].sigName)
	if err == nil {
		t.Fatalf("unexpected nont error")
	} else if !kerrors.IsNotFound(err) {
		t.Errorf("expected not found error, not: %#+v", err)
	}

	// delete the one in the middle
	err = userClient.ImageSignatures().Delete(image.Name + "@" + sigData[2].sigName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if image, err = userClient.Images().Get(image.Name); err != nil {
		t.Fatalf("unexpected error: %v", err)
	} else if len(image.Signatures) != 2 {
		t.Fatalf("expected 2 signatures, not %d", len(image.Signatures))
	}

	// delete the one at the end
	err = userClient.ImageSignatures().Delete(image.Name + "@" + sigData[3].sigName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// delete the last one
	err = userClient.ImageSignatures().Delete(image.Name + "@" + sigData[1].sigName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if image, err = userClient.Images().Get(image.Name); err != nil {
		t.Fatalf("unexpected error: %v", err)
	} else if len(image.Signatures) != 0 {
		t.Fatalf("expected 2 signatures, not %d", len(image.Signatures))
	}
}

func testSetupImageSignatureTest(t *testing.T, userName string) (adminClient *client.Client, userClient *client.Client, image *imageapi.Image) {
	testutil.RequireEtcd(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	adminClient, err = testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	image, err = testutil.GetImageFixture("testdata/test-image.json")
	if err != nil {
		t.Fatalf("failed to read image fixture: %v", err)
	}

	image, err = adminClient.Images().Create(image)
	if err != nil {
		t.Fatalf("unexpected error creating image: %v", err)
	}

	if len(image.Signatures) != 0 {
		t.Fatalf("expected empty signatures, not: %s", diff.ObjectDiff(image.Signatures, []imageapi.ImageSignature{}))
	}

	clusterAdminConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	userClient, _, _, err = testutil.GetClientForUser(*clusterAdminConfig, userName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return adminClient, userClient, image
}

func makeUserAnImageSigner(clusterAdminClient *client.Client, userClient *client.Client, userName string) error {
	// give bob permissions to update image signatures
	addImageSignerRole := &policy.RoleModificationOptions{
		RoleNamespace:       "",
		RoleName:            bootstrappolicy.ImageSignerRoleName,
		RoleBindingAccessor: policy.NewClusterRoleBindingAccessor(clusterAdminClient),
		Users:               []string{userName},
	}
	if err := addImageSignerRole.AddRole(); err != nil {
		return err
	}
	return testutil.WaitForClusterPolicyUpdate(userClient, "create", kapi.Resource("imagesignatures"), true)
}

func compareSignatures(t *testing.T, a, b imageapi.ImageSignature) {
	aName := a.Name
	a.ObjectMeta = b.ObjectMeta
	a.Name = aName
	if !reflect.DeepEqual(a, b) {
		t.Errorf("created and contained signatures differ: %v", diff.ObjectDiff(a, b))
	}
}
