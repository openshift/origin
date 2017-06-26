package testutil

import (
	"fmt"
	"sync"

	"github.com/docker/distribution/context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// FakeOpenShift is an in-mempory reactors for fake.Client.
type FakeOpenShift struct {
	logger context.Logger
	mu     sync.Mutex

	images       map[string]imageapi.Image
	imageStreams map[string]imageapi.ImageStream
}

// NewFakeOpenShift constructs the fake OpenShift reactors.
func NewFakeOpenShift() *FakeOpenShift {
	return &FakeOpenShift{
		logger: context.GetLogger(context.Background()),

		images:       make(map[string]imageapi.Image),
		imageStreams: make(map[string]imageapi.ImageStream),
	}
}

// NewFakeOpenShiftWithClient constructs a fake client associated with
// the stateful fake in-memory OpenShift reactors. The fake OpenShift is
// available for direct interaction, so you can make buggy states.
func NewFakeOpenShiftWithClient() (*FakeOpenShift, *testclient.Fake) {
	fos := NewFakeOpenShift()
	client := &testclient.Fake{}
	fos.AddReactorsTo(client)
	return fos, client
}

func (fos *FakeOpenShift) CreateImage(image *imageapi.Image) (*imageapi.Image, error) {
	fos.mu.Lock()
	defer fos.mu.Unlock()

	_, ok := fos.images[image.Name]
	if ok {
		return nil, errors.NewAlreadyExists(imageapi.Resource("images"), image.Name)
	}

	fos.images[image.Name] = *image
	fos.logger.Debugf("(*FakeOpenShift).images[%q] created", image.Name)

	return image, nil
}

func (fos *FakeOpenShift) GetImage(name string) (*imageapi.Image, error) {
	fos.mu.Lock()
	defer fos.mu.Unlock()

	image, ok := fos.images[name]
	if !ok {
		return nil, errors.NewNotFound(imageapi.Resource("images"), name)
	}

	return &image, nil
}

func (fos *FakeOpenShift) UpdateImage(image *imageapi.Image) (*imageapi.Image, error) {
	fos.mu.Lock()
	defer fos.mu.Unlock()

	_, ok := fos.images[image.Name]
	if !ok {
		return nil, errors.NewNotFound(imageapi.Resource("images"), image.Name)
	}

	fos.images[image.Name] = *image
	fos.logger.Debugf("(*FakeOpenShift).images[%q] updated", image.Name)

	return image, nil
}

func (fos *FakeOpenShift) CreateImageStream(namespace string, is *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	fos.mu.Lock()
	defer fos.mu.Unlock()

	ref := fmt.Sprintf("%s/%s", namespace, is.Name)

	_, ok := fos.imageStreams[ref]
	if ok {
		return nil, errors.NewAlreadyExists(imageapi.Resource("imagestreams"), is.Name)
	}

	is.Namespace = namespace
	is.CreationTimestamp = metav1.Now()

	fos.imageStreams[ref] = *is
	fos.logger.Debugf("(*FakeOpenShift).imageStreams[%q] created", ref)

	return is, nil
}

func (fos *FakeOpenShift) UpdateImageStream(namespace string, is *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	fos.mu.Lock()
	defer fos.mu.Unlock()

	ref := fmt.Sprintf("%s/%s", namespace, is.Name)

	oldis, ok := fos.imageStreams[ref]
	if !ok {
		return nil, errors.NewNotFound(imageapi.Resource("imagestreams"), is.Name)
	}

	is.Namespace = namespace
	is.CreationTimestamp = oldis.CreationTimestamp

	fos.imageStreams[ref] = *is
	fos.logger.Debugf("(*FakeOpenShift).imageStreams[%q] updated", ref)

	return is, nil
}

func (fos *FakeOpenShift) GetImageStream(namespace, repo string) (*imageapi.ImageStream, error) {
	fos.mu.Lock()
	defer fos.mu.Unlock()

	ref := fmt.Sprintf("%s/%s", namespace, repo)

	is, ok := fos.imageStreams[ref]
	if !ok {
		return nil, errors.NewNotFound(imageapi.Resource("imagestreams"), repo)
	}
	return &is, nil
}

func (fos *FakeOpenShift) CreateImageStreamMapping(namespace string, ism *imageapi.ImageStreamMapping) (*imageapi.ImageStreamMapping, error) {
	is, err := fos.GetImageStream(namespace, ism.Name)
	if err != nil {
		return nil, err
	}

	_, err = fos.CreateImage(&ism.Image)
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, err
	}

	if is.Status.Tags == nil {
		is.Status.Tags = make(map[string]imageapi.TagEventList)
	}

	tagEventList := is.Status.Tags[ism.Tag]
	tagEventList.Items = append(tagEventList.Items, imageapi.TagEvent{
		DockerImageReference: ism.Image.DockerImageReference,
		Image:                ism.Image.Name,
	})
	is.Status.Tags[ism.Tag] = tagEventList

	_, err = fos.UpdateImageStream(namespace, is)
	if err != nil {
		return nil, err
	}

	return ism, nil
}

func (fos *FakeOpenShift) CreateImageStreamTag(namespace string, istag *imageapi.ImageStreamTag) (*imageapi.ImageStreamTag, error) {
	imageStreamName, imageTag, ok := imageapi.SplitImageStreamTag(istag.Name)
	if !ok {
		return nil, fmt.Errorf("%q must be of the form <stream_name>:<tag>", istag.Name)
	}

	is, err := fos.GetImageStream(namespace, imageStreamName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}

		is = &imageapi.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name:      imageStreamName,
				Namespace: namespace,
			},
		}
	}

	if is.Spec.Tags == nil {
		is.Spec.Tags = make(map[string]imageapi.TagReference)
	}

	// The user wants to symlink a tag.
	_, exists := is.Spec.Tags[imageTag]
	if exists {
		return nil, errors.NewAlreadyExists(imageapi.Resource("imagestreamtag"), istag.Name)
	}
	is.Spec.Tags[imageTag] = *istag.Tag

	// TODO(dmage): use code from (pkg/image/registry/imagestream.Strategy).tagsChanged
	if is.Status.Tags == nil {
		is.Status.Tags = make(map[string]imageapi.TagEventList)
	}
	tagEventList := is.Status.Tags[imageTag]
	tagEventList.Items = append(
		[]imageapi.TagEvent{{
			Created:              istag.CreationTimestamp,
			DockerImageReference: istag.Image.DockerImageReference,
			Image:                istag.Image.Name,
			Generation:           istag.Generation,
		}},
		tagEventList.Items...,
	)
	is.Status.Tags[imageTag] = tagEventList

	// Check the stream creation timestamp and make sure we will not
	// create a new image stream while deleting.
	if is.CreationTimestamp.IsZero() {
		_, err = fos.CreateImageStream(namespace, is)
	} else {
		_, err = fos.UpdateImageStream(namespace, is)
	}
	if err != nil {
		return nil, err
	}

	return istag, nil
}

func (fos *FakeOpenShift) GetImageStreamImage(namespace string, id string) (*imageapi.ImageStreamImage, error) {
	name, imageID, err := imageapi.ParseImageStreamImageName(id)
	if err != nil {
		return nil, errors.NewBadRequest("ImageStreamImages must be retrieved with <name>@<id>")
	}

	repo, err := fos.GetImageStream(namespace, name)
	if err != nil {
		return nil, err
	}

	if repo.Status.Tags == nil {
		return nil, errors.NewNotFound(imageapi.Resource("imagestreamimage"), id)
	}

	event, err := imageapi.ResolveImageID(repo, imageID)
	if err != nil {
		return nil, err
	}

	imageName := event.Image
	image, err := fos.GetImage(imageName)
	if err != nil {
		return nil, err
	}
	if err := imageapi.ImageWithMetadata(image); err != nil {
		return nil, err
	}
	image.DockerImageManifest = ""
	image.DockerImageConfig = ""

	isi := imageapi.ImageStreamImage{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         namespace,
			Name:              imageapi.MakeImageStreamImageName(name, imageID),
			CreationTimestamp: image.ObjectMeta.CreationTimestamp,
			Annotations:       repo.Annotations,
		},
		Image: *image,
	}

	return &isi, nil
}

func (fos *FakeOpenShift) getName(action clientgotesting.Action) string {
	if getnamer, ok := action.(interface {
		GetName() string
	}); ok {
		return getnamer.GetName()
	}

	if getobjecter, ok := action.(interface {
		GetObject() runtime.Object
	}); ok {
		object := getobjecter.GetObject()
		if getnamer, ok := object.(interface {
			GetName() string
		}); ok {
			return getnamer.GetName()
		}
	}

	return "..."
}

func (fos *FakeOpenShift) log(msg string, f func() (bool, runtime.Object, error)) (bool, runtime.Object, error) {
	ok, obj, err := f()
	fos.logger.Debug(msg, ": err=", err)
	return ok, obj, err
}

func (fos *FakeOpenShift) todo(action clientgotesting.Action) (bool, runtime.Object, error) {
	return true, nil, fmt.Errorf("no reaction implemented for %v", action)
}

func (fos *FakeOpenShift) imagesHandler(action clientgotesting.Action) (bool, runtime.Object, error) {
	return fos.log(
		fmt.Sprintf("(*FakeOpenShift).imagesHandler: %s %s",
			action.GetVerb(), fos.getName(action)),
		func() (bool, runtime.Object, error) {
			switch action := action.(type) {
			case clientgotesting.GetActionImpl:
				image, err := fos.GetImage(action.Name)
				return true, image, err
			case clientgotesting.UpdateActionImpl:
				image, err := fos.UpdateImage(
					action.Object.(*imageapi.Image),
				)
				return true, image, err
			}
			return fos.todo(action)
		},
	)
}

func (fos *FakeOpenShift) imageStreamsHandler(action clientgotesting.Action) (bool, runtime.Object, error) {
	return fos.log(
		fmt.Sprintf("(*FakeOpenShift).imageStreamsHandler: %s %s/%s",
			action.GetVerb(), action.GetNamespace(), fos.getName(action)),
		func() (bool, runtime.Object, error) {
			switch action := action.(type) {
			case clientgotesting.CreateActionImpl:
				is, err := fos.CreateImageStream(
					action.GetNamespace(),
					action.Object.(*imageapi.ImageStream),
				)
				return true, is, err
			case clientgotesting.GetActionImpl:
				is, err := fos.GetImageStream(
					action.GetNamespace(),
					action.GetName(),
				)
				return true, is, err
			}
			return fos.todo(action)
		},
	)
}

func (fos *FakeOpenShift) imageStreamMappingsHandler(action clientgotesting.Action) (bool, runtime.Object, error) {
	return fos.log(
		fmt.Sprintf("(*FakeOpenShift).imageStreamMappingsHandler: %s %s/%s",
			action.GetVerb(), action.GetNamespace(), fos.getName(action)),
		func() (bool, runtime.Object, error) {
			switch action := action.(type) {
			case clientgotesting.CreateActionImpl:
				ism, err := fos.CreateImageStreamMapping(
					action.GetNamespace(),
					action.Object.(*imageapi.ImageStreamMapping),
				)
				return true, ism, err
			}
			return fos.todo(action)
		},
	)
}

func (fos *FakeOpenShift) imageStreamImagesHandler(action clientgotesting.Action) (bool, runtime.Object, error) {
	return fos.log(
		fmt.Sprintf("(*FakeOpenShift).imageStreamImagesHandler: %s %s/%s",
			action.GetVerb(), action.GetNamespace(), fos.getName(action)),
		func() (bool, runtime.Object, error) {
			switch action := action.(type) {
			case clientgotesting.GetActionImpl:
				isi, err := fos.GetImageStreamImage(
					action.GetNamespace(),
					action.GetName(),
				)
				return true, isi, err
			}
			return fos.todo(action)
		},
	)
}

// AddReactorsTo binds the reactors to client.
func (fos *FakeOpenShift) AddReactorsTo(client *testclient.Fake) {
	client.AddReactor("*", "images", fos.imagesHandler)
	client.AddReactor("*", "imagestreams", fos.imageStreamsHandler)
	client.AddReactor("*", "imagestreammappings", fos.imageStreamMappingsHandler)
	client.AddReactor("*", "imagestreamimages", fos.imageStreamImagesHandler)
}
