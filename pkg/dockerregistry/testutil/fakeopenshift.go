package testutil

import (
	"fmt"
	"sync"

	"github.com/docker/distribution/context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	imagefakeclient "github.com/openshift/origin/pkg/image/generated/clientset/typed/image/v1/fake"
)

// FakeOpenShift is an in-mempory reactors for fake.Client.
type FakeOpenShift struct {
	logger context.Logger
	mu     sync.Mutex

	images       map[string]imageapiv1.Image
	imageStreams map[string]imageapiv1.ImageStream
}

// NewFakeOpenShift constructs the fake OpenShift reactors.
func NewFakeOpenShift(ctx context.Context) *FakeOpenShift {
	return &FakeOpenShift{
		logger: context.GetLogger(ctx),

		images:       make(map[string]imageapiv1.Image),
		imageStreams: make(map[string]imageapiv1.ImageStream),
	}
}

// NewFakeOpenShiftWithClient constructs a fake client associated with
// the stateful fake in-memory OpenShift reactors. The fake OpenShift is
// available for direct interaction, so you can make buggy states.
func NewFakeOpenShiftWithClient(ctx context.Context) (*FakeOpenShift, *imagefakeclient.FakeImageV1) {
	fos := NewFakeOpenShift(ctx)

	imageClient := &imagefakeclient.FakeImageV1{Fake: &clientgotesting.Fake{}}
	fos.AddReactorsTo(imageClient)
	return fos, imageClient
}

func (fos *FakeOpenShift) CreateImage(image *imageapiv1.Image) (*imageapiv1.Image, error) {
	fos.mu.Lock()
	defer fos.mu.Unlock()

	_, ok := fos.images[image.Name]
	if ok {
		return nil, errors.NewAlreadyExists(imageapiv1.Resource("images"), image.Name)
	}

	fos.images[image.Name] = *image
	fos.logger.Debugf("(*FakeOpenShift).images[%q] created", image.Name)

	return image, nil
}

func (fos *FakeOpenShift) GetImage(name string) (*imageapiv1.Image, error) {
	fos.mu.Lock()
	defer fos.mu.Unlock()
	image, ok := fos.images[name]
	if !ok {
		return nil, errors.NewNotFound(imageapiv1.Resource("images"), name)
	}

	return &image, nil
}

func (fos *FakeOpenShift) UpdateImage(image *imageapiv1.Image) (*imageapiv1.Image, error) {
	fos.mu.Lock()
	defer fos.mu.Unlock()

	_, ok := fos.images[image.Name]
	if !ok {
		return nil, errors.NewNotFound(imageapiv1.Resource("images"), image.Name)
	}

	fos.images[image.Name] = *image
	fos.logger.Debugf("(*FakeOpenShift).images[%q] updated", image.Name)

	return image, nil
}

func (fos *FakeOpenShift) CreateImageStream(namespace string, is *imageapiv1.ImageStream) (*imageapiv1.ImageStream, error) {
	fos.mu.Lock()
	defer fos.mu.Unlock()

	ref := fmt.Sprintf("%s/%s", namespace, is.Name)

	_, ok := fos.imageStreams[ref]
	if ok {
		return nil, errors.NewAlreadyExists(imageapiv1.Resource("imagestreams"), is.Name)
	}

	is.Namespace = namespace
	is.CreationTimestamp = metav1.Now()

	fos.imageStreams[ref] = *is
	fos.logger.Debugf("(*FakeOpenShift).imageStreams[%q] created", ref)

	return is, nil
}

func (fos *FakeOpenShift) UpdateImageStream(namespace string, is *imageapiv1.ImageStream) (*imageapiv1.ImageStream, error) {
	fos.mu.Lock()
	defer fos.mu.Unlock()

	ref := fmt.Sprintf("%s/%s", namespace, is.Name)

	oldis, ok := fos.imageStreams[ref]
	if !ok {
		return nil, errors.NewNotFound(imageapiv1.Resource("imagestreams"), is.Name)
	}

	is.Namespace = namespace
	is.CreationTimestamp = oldis.CreationTimestamp

	fos.imageStreams[ref] = *is
	fos.logger.Debugf("(*FakeOpenShift).imageStreams[%q] updated", ref)

	return is, nil
}

func (fos *FakeOpenShift) GetImageStream(namespace, repo string) (*imageapiv1.ImageStream, error) {
	fos.mu.Lock()
	defer fos.mu.Unlock()

	ref := fmt.Sprintf("%s/%s", namespace, repo)

	is, ok := fos.imageStreams[ref]
	if !ok {
		return nil, errors.NewNotFound(imageapiv1.Resource("imagestreams"), repo)
	}
	return &is, nil
}

func (fos *FakeOpenShift) CreateImageStreamMapping(namespace string, ism *imageapiv1.ImageStreamMapping) (*imageapiv1.ImageStreamMapping, error) {
	is, err := fos.GetImageStream(namespace, ism.Name)
	if err != nil {
		return nil, err
	}

	_, err = fos.CreateImage(&ism.Image)
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, err
	}

	var tagEventList *imageapiv1.NamedTagEventList
	for i, t := range is.Status.Tags {
		if t.Tag == ism.Tag {
			tagEventList = &is.Status.Tags[i]
			break
		}
	}
	if tagEventList == nil {
		is.Status.Tags = append(is.Status.Tags, imageapiv1.NamedTagEventList{
			Tag: ism.Tag,
		})
		tagEventList = &is.Status.Tags[len(is.Status.Tags)-1]
	}

	tagEventList.Items = append(tagEventList.Items, imageapiv1.TagEvent{
		DockerImageReference: ism.Image.DockerImageReference,
		Image:                ism.Image.Name,
	})

	_, err = fos.UpdateImageStream(namespace, is)
	if err != nil {
		return nil, err
	}

	return ism, nil
}

func (fos *FakeOpenShift) CreateImageStreamTag(namespace string, istag *imageapiv1.ImageStreamTag) (*imageapiv1.ImageStreamTag, error) {
	imageStreamName, imageTag, ok := imageapi.SplitImageStreamTag(istag.Name)
	if !ok {
		return nil, fmt.Errorf("%q must be of the form <stream_name>:<tag>", istag.Name)
	}

	is, err := fos.GetImageStream(namespace, imageStreamName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}

		is = &imageapiv1.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name:      imageStreamName,
				Namespace: namespace,
			},
		}
	}

	// The user wants to symlink a tag.
	for _, t := range is.Spec.Tags {
		if t.Name == imageTag {
			return nil, errors.NewAlreadyExists(imageapiv1.Resource("imagestreamtag"), istag.Name)
		}
	}
	is.Spec.Tags = append(is.Spec.Tags, *istag.Tag)

	// TODO(dmage): use code from (pkg/image/registry/imagestream.Strategy).tagsChanged
	var (
		updatedNamedList *imageapiv1.NamedTagEventList
		position         int
	)
	for i, t := range is.Status.Tags {
		if t.Tag == imageTag {
			updatedNamedList = &t
			position = i
			break
		}
	}
	if updatedNamedList != nil {
		updatedNamedList.Items = append(updatedNamedList.Items, imageapiv1.TagEvent{
			Created:              istag.CreationTimestamp,
			DockerImageReference: istag.Image.DockerImageReference,
			Image:                istag.Image.Name,
			Generation:           istag.Generation,
		})
		is.Status.Tags[position] = *updatedNamedList
	} else {
		is.Status.Tags = append(is.Status.Tags, imageapiv1.NamedTagEventList{
			Tag: imageTag,
			Items: []imageapiv1.TagEvent{{
				Created:              istag.CreationTimestamp,
				DockerImageReference: istag.Image.DockerImageReference,
				Image:                istag.Image.Name,
				Generation:           istag.Generation,
			}},
		})
	}

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

func (fos *FakeOpenShift) GetImageStreamImage(namespace string, id string) (*imageapiv1.ImageStreamImage, error) {
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

	event, err := imageapiv1.ResolveImageID(repo, imageID)
	if err != nil {
		return nil, err
	}

	imageName := event.Image
	image, err := fos.GetImage(imageName)
	if err != nil {
		return nil, err
	}
	if err := imageapiv1.ImageWithMetadata(image); err != nil {
		return nil, err
	}
	image.DockerImageManifest = ""
	image.DockerImageConfig = ""

	isi := imageapiv1.ImageStreamImage{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         namespace,
			Name:              imageapi.JoinImageStreamImage(name, imageID),
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
					action.Object.(*imageapiv1.Image),
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
					action.Object.(*imageapiv1.ImageStream),
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
				_, err := fos.CreateImageStreamMapping(
					action.GetNamespace(),
					action.Object.(*imageapiv1.ImageStreamMapping),
				)
				return true, &metav1.Status{}, err
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
func (fos *FakeOpenShift) AddReactorsTo(c *imagefakeclient.FakeImageV1) {
	c.AddReactor("*", "images", fos.imagesHandler)
	c.AddReactor("*", "imagestreams", fos.imageStreamsHandler)
	c.AddReactor("*", "imagestreammappings", fos.imageStreamMappingsHandler)
	c.AddReactor("*", "imagestreamimages", fos.imageStreamImagesHandler)
}
