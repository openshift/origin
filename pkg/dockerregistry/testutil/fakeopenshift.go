package testutil

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/docker/distribution/context"

	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// FakeOpenShift is an in-mempory reactors for fake.Client.
type FakeOpenShift struct {
	logger context.Logger

	imagesStorage       map[string]*imageapi.Image
	imageStreamsStorage map[string]*imageapi.ImageStream
}

// NewFakeOpenShift constructs the fake OpenShift reactors.
func NewFakeOpenShift() *FakeOpenShift {
	return &FakeOpenShift{
		logger: context.GetLogger(context.Background()),

		imagesStorage:       make(map[string]*imageapi.Image),
		imageStreamsStorage: make(map[string]*imageapi.ImageStream),
	}
}

// NewFakeOpenShiftWithClient constructs a fake client associated with
// the stateful fake in-memory OpenShift reactors. The fake OpenShift is
// available for direct interaction, so you can make buggy states.
func NewFakeOpenShiftWithClient() (*FakeOpenShift, *testclient.Fake) {
	os := NewFakeOpenShift()
	client := &testclient.Fake{}
	os.AddReactorsTo(client)
	return os, client
}

func (os *FakeOpenShift) CreateImage(image *imageapi.Image) (*imageapi.Image, error) {
	_, ok := os.imagesStorage[image.Name]
	if ok {
		return nil, errors.NewAlreadyExists(unversioned.GroupResource{
			Group:    "",
			Resource: "images",
		}, image.Name)
	}

	os.logger.Debug("(*FakeOpenShift).CreateImage: ", image.Name)
	os.imagesStorage[image.Name] = image
	return image, nil
}

func (os *FakeOpenShift) GetImage(name string) (*imageapi.Image, error) {
	image, ok := os.imagesStorage[name]
	if !ok {
		return nil, errors.NewNotFound(unversioned.GroupResource{
			Group:    "",
			Resource: "images",
		}, name)
	}
	return image, nil
}

func (os *FakeOpenShift) CreateImageStream(namespace string, is *imageapi.ImageStream) (*imageapi.ImageStream, error) {
	ref := fmt.Sprintf("%s/%s", namespace, is.Name)

	_, ok := os.imageStreamsStorage[ref]
	if ok {
		return nil, errors.NewAlreadyExists(unversioned.GroupResource{
			Group:    "",
			Resource: "imagestreams",
		}, is.Name)
	}

	is.Namespace = namespace

	os.logger.Debug("(*FakeOpenShift).CreateImageStream: ", ref)
	os.imageStreamsStorage[ref] = is
	return is, nil
}

func (os *FakeOpenShift) GetImageStream(namespace, repo string) (*imageapi.ImageStream, error) {
	ref := fmt.Sprintf("%s/%s", namespace, repo)

	is, ok := os.imageStreamsStorage[ref]
	if !ok {
		return nil, errors.NewNotFound(unversioned.GroupResource{
			Group:    "",
			Resource: "imagestreams",
		}, repo)
	}
	return is, nil
}

func (os *FakeOpenShift) CreateImageStreamMapping(group string, ism *imageapi.ImageStreamMapping) (*imageapi.ImageStreamMapping, error) {
	is, err := os.GetImageStream(group, ism.ObjectMeta.Name)
	if err != nil {
		return nil, err
	}

	_, err = os.CreateImage(&ism.Image)
	if err != nil {
		return nil, err
	}

	if is.Status.Tags == nil {
		is.Status.Tags = make(map[string]imageapi.TagEventList)
	}

	tagEventList := is.Status.Tags[ism.Tag]
	tagEventList.Items = append(tagEventList.Items, imageapi.TagEvent{
		Image: ism.Image.Name,
	})
	is.Status.Tags[ism.Tag] = tagEventList

	return ism, nil
}

func (os *FakeOpenShift) getName(action core.Action) string {
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

func (os *FakeOpenShift) log(msg string, f func() (bool, runtime.Object, error)) (bool, runtime.Object, error) {
	ok, obj, err := f()
	os.logger.Debug(msg, ": err=", err)
	return ok, obj, err
}

func (os *FakeOpenShift) todo(action core.Action) (bool, runtime.Object, error) {
	return true, nil, fmt.Errorf("no reaction implemented for %v", action)
}

func (os *FakeOpenShift) images(action core.Action) (bool, runtime.Object, error) {
	return os.log(
		fmt.Sprintf("(*FakeOpenShift).images: %s %s",
			action.GetVerb(), os.getName(action)),
		func() (bool, runtime.Object, error) {
			switch action := action.(type) {
			case core.GetActionImpl:
				image, err := os.GetImage(action.Name)
				return true, image, err
			}
			return os.todo(action)
		},
	)
}

func (os *FakeOpenShift) imageStreams(action core.Action) (bool, runtime.Object, error) {
	return os.log(
		fmt.Sprintf("(*FakeOpenShift).imageStreams: %s %s/%s",
			action.GetVerb(), action.GetNamespace(), os.getName(action)),
		func() (bool, runtime.Object, error) {
			switch action := action.(type) {
			case core.CreateActionImpl:
				is, err := os.CreateImageStream(
					action.GetNamespace(),
					action.Object.(*imageapi.ImageStream),
				)
				return true, is, err
			case core.GetActionImpl:
				is, err := os.GetImageStream(
					action.GetNamespace(),
					action.GetName(),
				)
				return true, is, err
			}
			return os.todo(action)
		},
	)
}

func (os *FakeOpenShift) imageStreamMappings(action core.Action) (bool, runtime.Object, error) {
	return os.log(
		fmt.Sprintf("(*FakeOpenShift).imageStreamMappings: %s %s/%s",
			action.GetVerb(), action.GetNamespace(), os.getName(action)),
		func() (bool, runtime.Object, error) {
			switch action := action.(type) {
			case core.CreateActionImpl:
				ism, err := os.CreateImageStreamMapping(
					action.GetNamespace(),
					action.Object.(*imageapi.ImageStreamMapping),
				)
				return true, ism, err
			}
			return os.todo(action)
		},
	)
}

// AddReactorsTo binds the reactors to client.
func (os *FakeOpenShift) AddReactorsTo(client *testclient.Fake) {
	client.AddReactor("*", "images", os.images)
	client.AddReactor("*", "imagestreams", os.imageStreams)
	client.AddReactor("*", "imagestreammappings", os.imageStreamMappings)
}

// CreateRandomImage creates an image with a random content.
func CreateRandomImage(namespace, name string) (*imageapi.Image, error) {
	_, manifest, _, err := CreateRandomManifest(ManifestSchema1, 3)
	if err != nil {
		return nil, err
	}

	_, manifestSchema1, err := manifest.Payload()
	if err != nil {
		return nil, err
	}

	image, err := NewImageForManifest(
		fmt.Sprintf("%s/%s", namespace, name),
		string(manifestSchema1),
		"",
		false,
	)
	if err != nil {
		return nil, err
	}

	return image, nil
}

// RegisterImage adds image to the image stream namespace/name.
func RegisterImage(os *FakeOpenShift, image *imageapi.Image, namespace, name, tag string) error {
	is, err := os.GetImageStream(namespace, name)
	if err != nil {
		is = &imageapi.ImageStream{}
		is.Name = name
		is.Annotations = map[string]string{
			imageapi.InsecureRepositoryAnnotation: "true",
		}

		is, err = os.CreateImageStream(namespace, is)
		if err != nil {
			return err
		}
	}

	_, err = os.CreateImageStreamMapping(namespace, &imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
		Image: *image,
		Tag:   tag,
	})
	if err != nil {
		return err
	}

	return nil
}

// RegisterRandomImage adds image with a random content to the image stream namespace/name.
func RegisterRandomImage(os *FakeOpenShift, namespace, name, tag string) (*imageapi.Image, error) {
	image, err := CreateRandomImage(namespace, name)
	if err != nil {
		return nil, err
	}

	return image, RegisterImage(os, image, namespace, name, tag)
}
