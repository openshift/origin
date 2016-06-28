package server

import (
	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"

	kapi "k8s.io/kubernetes/pkg/api"

	imageapi "github.com/openshift/origin/pkg/image/api"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

type tagService struct {
	distribution.TagService

	repo *repository
}

func (t tagService) Get(ctx context.Context, tag string) (distribution.Descriptor, error) {
	imageStream, err := t.repo.getImageStream()
	if err != nil {
		context.GetLogger(ctx).Errorf("error retrieving ImageStream %s/%s: %v", t.repo.namespace, t.repo.name, err)
		return distribution.Descriptor{}, distribution.ErrRepositoryUnknown{Name: t.repo.Named().Name()}
	}

	te := imageapi.LatestTaggedImage(imageStream, tag)
	if te == nil {
		return distribution.Descriptor{}, distribution.ErrTagUnknown{Tag: tag}
	}
	dgst, err := digest.ParseDigest(te.Image)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	if !t.repo.pullthrough {
		image, err := t.repo.getImage(dgst)
		if err != nil {
			return distribution.Descriptor{}, err
		}

		if !isImageManaged(image) {
			return distribution.Descriptor{}, distribution.ErrTagUnknown{Tag: tag}
		}
	}

	return distribution.Descriptor{Digest: dgst}, nil
}

func (t tagService) All(ctx context.Context) ([]string, error) {
	tags := []string{}

	imageStream, err := t.repo.getImageStream()
	if err != nil {
		context.GetLogger(ctx).Errorf("error retrieving ImageStream %s/%s: %v", t.repo.namespace, t.repo.name, err)
		return tags, distribution.ErrRepositoryUnknown{Name: t.repo.Named().Name()}
	}

	managedImages := make(map[string]bool)

	for tag, history := range imageStream.Status.Tags {
		if len(history.Items) == 0 {
			continue
		}

		if t.repo.pullthrough {
			tags = append(tags, tag)
			continue
		}

		managed, found := managedImages[history.Items[0].Image]
		if !found {
			dgst, err := digest.ParseDigest(history.Items[0].Image)
			if err != nil {
				context.GetLogger(ctx).Errorf("bad digest %s: %v", history.Items[0].Image, err)
				continue
			}

			image, err := t.repo.getImage(dgst)
			if err != nil {
				context.GetLogger(ctx).Errorf("unable to get image %s/%s %s: %v", t.repo.namespace, t.repo.name, dgst.String(), err)
				continue
			}
			managed = isImageManaged(image)
			managedImages[history.Items[0].Image] = managed
		}

		if !managed {
			continue
		}

		tags = append(tags, tag)
	}
	return tags, nil
}

func (t tagService) Lookup(ctx context.Context, desc distribution.Descriptor) ([]string, error) {
	tags := []string{}

	imageStream, err := t.repo.getImageStream()
	if err != nil {
		context.GetLogger(ctx).Errorf("error retrieving ImageStream %s/%s: %v", t.repo.namespace, t.repo.name, err)
		return tags, distribution.ErrRepositoryUnknown{Name: t.repo.Named().Name()}
	}

	managedImages := make(map[string]bool)

	for tag, history := range imageStream.Status.Tags {
		if len(history.Items) == 0 {
			continue
		}

		dgst, err := digest.ParseDigest(history.Items[0].Image)
		if err != nil {
			context.GetLogger(ctx).Errorf("bad digest %s: %v", history.Items[0].Image, err)
			continue
		}

		if dgst != desc.Digest {
			continue
		}

		if t.repo.pullthrough {
			tags = append(tags, tag)
			continue
		}

		managed, found := managedImages[history.Items[0].Image]
		if !found {
			image, err := t.repo.getImage(dgst)
			if err != nil {
				context.GetLogger(ctx).Errorf("unable to get image %s/%s %s: %v", t.repo.namespace, t.repo.name, dgst.String(), err)
				continue
			}
			managed = isImageManaged(image)
			managedImages[history.Items[0].Image] = managed
		}

		if !managed {
			continue
		}

		tags = append(tags, tag)
	}

	return tags, nil
}

func (t tagService) Tag(ctx context.Context, tag string, dgst distribution.Descriptor) error {
	imageStream, err := t.repo.getImageStream()
	if err != nil {
		context.GetLogger(ctx).Errorf("error retrieving ImageStream %s/%s: %v", t.repo.namespace, t.repo.name, err)
		return distribution.ErrRepositoryUnknown{Name: t.repo.Named().Name()}
	}

	image, err := t.repo.registryOSClient.Images().Get(dgst.Digest.String())
	if err != nil {
		context.GetLogger(ctx).Errorf("unable to get image: %s", dgst.Digest.String())
		return err
	}
	image.SetResourceVersion("")

	if !t.repo.pullthrough && !isImageManaged(image) {
		return distribution.ErrRepositoryUnknown{Name: t.repo.Named().Name()}
	}

	ism := imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: imageStream.Namespace,
			Name:      imageStream.Name,
		},
		Tag:   tag,
		Image: *image,
	}

	err = t.repo.registryOSClient.ImageStreamMappings(imageStream.Namespace).Create(&ism)
	if quotautil.IsErrorQuotaExceeded(err) {
		context.GetLogger(ctx).Errorf("denied creating ImageStreamMapping: %v", err)
		return distribution.ErrAccessDenied
	}

	return err
}

func (t tagService) Untag(ctx context.Context, tag string) error {
	imageStream, err := t.repo.getImageStream()
	if err != nil {
		context.GetLogger(ctx).Errorf("error retrieving ImageStream %s/%s: %v", t.repo.namespace, t.repo.name, err)
		return distribution.ErrRepositoryUnknown{Name: t.repo.Named().Name()}
	}

	te := imageapi.LatestTaggedImage(imageStream, tag)
	if te == nil {
		return distribution.ErrTagUnknown{Tag: tag}
	}

	if !t.repo.pullthrough {
		dgst, err := digest.ParseDigest(te.Image)
		if err != nil {
			return err
		}

		image, err := t.repo.getImage(dgst)
		if err != nil {
			return err
		}

		if !isImageManaged(image) {
			return distribution.ErrTagUnknown{Tag: tag}
		}
	}

	return t.repo.registryOSClient.ImageStreamTags(imageStream.Namespace).Delete(imageStream.Name, tag)
}

func isImageManaged(image *imageapi.Image) bool {
	managed, ok := image.ObjectMeta.Annotations[imageapi.ManagedByOpenShiftAnnotation]
	return ok && managed == "true"
}
