package signature

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/controller"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imagefake "github.com/openshift/origin/pkg/image/generated/internalclientset/fake"
)

var (
	noSignatures        = []imageapi.ImageSignature{}
	singleFakeSignature = []imageapi.ImageSignature{{
		ObjectMeta: metav1.ObjectMeta{Name: "fake@1111111"},
		Content:    []byte(`fake`),
	}}
	multipleSignatures = []imageapi.ImageSignature{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "fake@1111111"},
			Content:    []byte(`fake`),
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "fake@2222222"},
			Content:    []byte(`fake`),
		},
	}
)

func TestSignatureImport(t *testing.T) {
	testCases := []struct {
		name               string
		image              *imageapi.Image
		expectNoUpdate     bool
		limit              int
		expect, signatures []imageapi.ImageSignature
	}{
		{
			name:           "no-op",
			image:          makeImage("img1", "foo.bar/test@sha256:1111", noSignatures),
			expectNoUpdate: true,
			limit:          3,
			signatures:     noSignatures,
		},
		{
			name:           "existing",
			image:          makeImage("img2", "foo.bar/test@sha256:2222", singleFakeSignature),
			expectNoUpdate: true,
			limit:          3,
			signatures:     singleFakeSignature,
		},
		{
			name:       "success",
			image:      makeImage("img2", "foo.bar/test@sha256:2222", noSignatures),
			limit:      3,
			signatures: singleFakeSignature,
			expect:     singleFakeSignature,
		},
		{
			name:           "reached limit",
			image:          makeImage("img3", "foo.bar/test@sha256:2222", singleFakeSignature),
			limit:          1,
			expectNoUpdate: true,
			signatures:     multipleSignatures,
			expect:         singleFakeSignature,
		},
		{
			name:       "multiple with limit",
			image:      makeImage("img3", "foo.bar/test@sha256:2222", noSignatures),
			limit:      2,
			signatures: multipleSignatures,
			expect:     multipleSignatures,
		},
		{
			name:           "no-op with zero limit",
			image:          makeImage("img3", "foo.bar/test@sha256:2222", singleFakeSignature),
			limit:          0,
			expectNoUpdate: true,
			signatures:     multipleSignatures,
			expect:         noSignatures,
		},
	}

	for _, tc := range testCases {
		stopChannel := make(chan struct{})
		fetchChannel := make(chan struct{})
		updateChannel := make(chan *imageapi.Image)
		client, _, controller, factory := controllerSetup([]runtime.Object{tc.image}, t, tc.limit, stopChannel)
		client.PrependReactor("update", "images", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
			obj := action.(ktesting.UpdateAction).GetObject()
			updateChannel <- obj.(*imageapi.Image)
			return true, obj, nil
		})

		controller.fetcher = newSignatureRetriever(tc.signatures, fetchChannel)
		factory.Start(stopChannel)
		go controller.Run(5, stopChannel)

		// Wait for the fetch to happen
		select {
		case <-fetchChannel:
		case <-time.After(time.Duration(10 * time.Second)):
			t.Fatalf("[%s] failed to wait for fetch to happen", tc.name)
		}

		if tc.expectNoUpdate {
			select {
			case <-updateChannel:
				t.Errorf("[%s] unexpected update", tc.name)
			case <-time.After(time.Duration(5 * time.Second)):
				close(stopChannel)
				continue
			}
		}

		select {
		case updatedImage := <-updateChannel:
			if len(updatedImage.Signatures) != len(tc.expect) {
				t.Errorf("[%s] expected %d signatures, got %d", tc.name, len(tc.expect), len(updatedImage.Signatures))
			}
		case <-time.After(time.Duration(5 * time.Second)):
			t.Fatalf("[%s] failed to wait for update to happen", tc.name)
		}

		close(stopChannel)
	}
}

type fakeSignatureRetriever struct {
	signatures  []imageapi.ImageSignature
	fetchCalled chan struct{}
}

func newSignatureRetriever(s []imageapi.ImageSignature, ch chan struct{}) *fakeSignatureRetriever {
	return &fakeSignatureRetriever{signatures: s, fetchCalled: ch}
}

func (f *fakeSignatureRetriever) DownloadImageSignatures(image *imageapi.Image) ([]imageapi.ImageSignature, error) {
	close(f.fetchCalled)
	return f.signatures, nil
}

func controllerSetup(startingImages []runtime.Object, t *testing.T, limit int, stopCh <-chan struct{}) (*imagefake.Clientset, *watch.FakeWatcher, *SignatureImportController, imageinformer.SharedInformerFactory) {
	imageclient := imagefake.NewSimpleClientset(startingImages...)
	fakeWatch := watch.NewFake()
	informerFactory := imageinformer.NewSharedInformerFactory(imageclient, controller.NoResyncPeriodFunc())

	controller := NewSignatureImportController(
		context.Background(),
		imageclient,
		informerFactory.Image().InternalVersion().Images(),
		30*time.Second,
		10*time.Second,
		limit,
	)
	controller.imageHasSynced = func() bool { return true }

	return imageclient, fakeWatch, controller, informerFactory
}

func makeImage(name, dockerRef string, signatures []imageapi.ImageSignature) *imageapi.Image {
	i := imageapi.Image{}
	i.Name = name
	i.DockerImageReference = dockerRef
	i.Signatures = signatures
	return &i
}
