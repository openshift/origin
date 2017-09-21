package server

import (
	"fmt"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api/v1"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
)

type tagService struct {
	distribution.TagService

	repo *repository
}

func (t tagService) Get(ctx context.Context, tag string) (distribution.Descriptor, error) {
	imageStream, err := t.repo.imageStreamGetter.get()
	if err != nil {
		context.GetLogger(ctx).Errorf("error retrieving ImageStream %s/%s: %v", t.repo.namespace, t.repo.name, err)
		return distribution.Descriptor{}, distribution.ErrRepositoryUnknown{Name: t.repo.Named().Name()}
	}

	te := imageapiv1.LatestTaggedImage(imageStream, tag)
	if te == nil {
		return distribution.Descriptor{}, distribution.ErrTagUnknown{Tag: tag}
	}
	dgst, err := digest.ParseDigest(te.Image)
	if err != nil {
		return distribution.Descriptor{}, err
	}

	if !t.repo.config.pullthrough {
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

	imageStream, err := t.repo.imageStreamGetter.get()
	if err != nil {
		context.GetLogger(ctx).Errorf("error retrieving ImageStream %s/%s: %v", t.repo.namespace, t.repo.name, err)
		return tags, distribution.ErrRepositoryUnknown{Name: t.repo.Named().Name()}
	}

	managedImages := make(map[string]bool)

	for _, history := range imageStream.Status.Tags {
		if len(history.Items) == 0 {
			continue
		}
		tag := history.Tag

		if t.repo.config.pullthrough {
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

	imageStream, err := t.repo.imageStreamGetter.get()
	if err != nil {
		context.GetLogger(ctx).Errorf("error retrieving ImageStream %s/%s: %v", t.repo.namespace, t.repo.name, err)
		return tags, distribution.ErrRepositoryUnknown{Name: t.repo.Named().Name()}
	}

	managedImages := make(map[string]bool)

	for _, history := range imageStream.Status.Tags {
		if len(history.Items) == 0 {
			continue
		}
		tag := history.Tag

		dgst, err := digest.ParseDigest(history.Items[0].Image)
		if err != nil {
			context.GetLogger(ctx).Errorf("bad digest %s: %v", history.Items[0].Image, err)
			continue
		}

		if dgst != desc.Digest {
			continue
		}

		if t.repo.config.pullthrough {
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
	dockerImageReference := fmt.Sprintf("%s/%s/%s@%s", t.repo.config.registryAddr, t.repo.namespace, t.repo.name, dgst.Digest.String())

	isi := imageapiv1.ImageStreamImport{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: t.repo.namespace,
			Name:      t.repo.name,
		},
		Spec: imageapiv1.ImageStreamImportSpec{
			Import: true,
			Images: []imageapiv1.ImageImportSpec{
				{
					From: kapi.ObjectReference{Kind: "DockerImage", Name: dockerImageReference},
					To:   &kapi.LocalObjectReference{Name: tag},

					ImportPolicy: imageapiv1.TagImportPolicy{
						Insecure: true,
					},
				},
			},
		},
	}

	userClient, ok := userClientFrom(ctx)
	if !ok {
		return fmt.Errorf("no user client in context")
	}

	if _, err := userClient.ImageStreamImports(t.repo.namespace).Create(&isi); err != nil {
		return fmt.Errorf("image stream import: %s", err)
	}

	return nil
}

func (t tagService) Untag(ctx context.Context, tag string) error {
	imageStream, err := t.repo.imageStreamGetter.get()
	if err != nil {
		context.GetLogger(ctx).Errorf("error retrieving ImageStream %s/%s: %v", t.repo.namespace, t.repo.name, err)
		return distribution.ErrRepositoryUnknown{Name: t.repo.Named().Name()}
	}

	te := imageapiv1.LatestTaggedImage(imageStream, tag)
	if te == nil {
		return distribution.ErrTagUnknown{Tag: tag}
	}

	if !t.repo.config.pullthrough {
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

	return t.repo.registryOSClient.ImageStreamTags(imageStream.Namespace).Delete(imageapi.JoinImageStreamTag(imageStream.Name, tag), &metav1.DeleteOptions{})
}
